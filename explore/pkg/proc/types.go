package proc

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"go/constant"
	"reflect"

	"explore/pkg/dwarf/godwarf"
	"explore/pkg/dwarf/reader"
)

// The kind field in runtime._type is a reflect.Kind value plus
// some extra flags defined here.
// See equivalent declaration in $GOROOT/src/reflect/type.go
const (
	kindDirectIface = 1 << 5 // +rtype kindDirectIface|internal/abi.KindDirectIface
	kindGCProg      = 1 << 6 // +rtype kindGCProg|internal/abi.KindGCProg
	kindNoPointers  = 1 << 7
	kindMask        = (1 << 5) - 1 // +rtype kindMask|internal/abi.KindMask
)

type runtimeTypeDIE struct {
	offset dwarf.Offset
	kind   int64
}

func pointerTo(typ godwarf.Type, arch *Arch) godwarf.Type {
	return &godwarf.PtrType{
		CommonType: godwarf.CommonType{
			ByteSize:    int64(arch.PtrSize()),
			Name:        "*" + typ.Common().Name,
			ReflectKind: reflect.Ptr,
			Offset:      0,
		},
		Type: typ,
	}
}

type loadDebugInfoMapsContext struct {
	ardr                *reader.Reader
	abstractOriginTable map[dwarf.Offset]int
	knownPackageVars    map[string]struct{}
	offsetToVersion     map[dwarf.Offset]uint8
}

func newLoadDebugInfoMapsContext(bi *BinaryInfo, image *Image, offsetToVersion map[dwarf.Offset]uint8) *loadDebugInfoMapsContext {
	ctxt := &loadDebugInfoMapsContext{}

	ctxt.ardr = image.DwarfReader()
	ctxt.abstractOriginTable = make(map[dwarf.Offset]int)
	ctxt.offsetToVersion = offsetToVersion

	ctxt.knownPackageVars = map[string]struct{}{}
	for _, v := range bi.packageVars {
		ctxt.knownPackageVars[v.name] = struct{}{}
	}

	return ctxt
}

func (ctxt *loadDebugInfoMapsContext) lookupAbstractOrigin(bi *BinaryInfo, off dwarf.Offset) int {
	r, ok := ctxt.abstractOriginTable[off]
	if !ok {
		bi.Functions = append(bi.Functions, Function{})
		r = len(bi.Functions) - 1
		bi.Functions[r].offset = off
		ctxt.abstractOriginTable[off] = r
	}
	return r
}

// RuntimeTypeToDIE returns the DIE corresponding to the runtime._type.
// This is done in three different ways depending on the version of go.
//   - Before go1.7 the type name is retrieved directly from the runtime._type
//     and looked up in debug_info
//   - After go1.7 the runtime._type struct is read recursively to reconstruct
//     the name of the type, and then the type's name is used to look up
//     debug_info
//   - After go1.11 the runtimeTypeToDIE map is used to look up the address of
//     the type and map it directly to a DIE.
func RuntimeTypeToDIE(_type *Variable, dataAddr uint64, mds []ModuleData) (typ godwarf.Type, kind int64, err error) {
	bi := _type.bi

	_type = _type.maybeDereference()

	// go 1.11 implementation: use extended attribute in debug_info

	md := findModuleDataForType(mds, _type.Addr)
	if md != nil {
		so := bi.moduleDataToImage(md)
		if so != nil {
			if rtdie, ok := so.runtimeTypeToDIE[_type.Addr-md.types]; ok {
				typ, err := godwarf.ReadType(so.dwarf, so.index, rtdie.offset, so.typeCache)
				if err != nil {
					return nil, 0, fmt.Errorf("invalid interface type: %v", err)
				}
				if rtdie.kind == -1 {
					if kindField := _type.loadFieldNamed("kind"); kindField != nil && kindField.Value != nil {
						rtdie.kind, _ = constant.Int64Val(kindField.Value)
					} else if kindField := _type.loadFieldNamed("Kind_"); kindField != nil && kindField.Value != nil {
						rtdie.kind, _ = constant.Int64Val(kindField.Value)
					}
				}
				return typ, rtdie.kind, nil
			}
		}
	}

	return nil, 0, errors.New("could not resolve interface type")
}

// resolveParametricType returns the real type of t if t is a parametric
// type, by reading the correct dictionary entry.
func resolveParametricType(bi *BinaryInfo, mem MemoryReadWriter, t godwarf.Type, dictAddr uint64) (godwarf.Type, error) {
	ptyp, _ := t.(*godwarf.ParametricType)
	if ptyp == nil {
		return t, nil
	}
	if dictAddr == 0 {
		return ptyp.TypedefType.Type, errors.New("parametric type without a dictionary")
	}
	rtypeAddr, err := readUintRaw(mem, dictAddr+uint64(ptyp.DictIndex*int64(bi.Arch.PtrSize())), int64(bi.Arch.PtrSize()))
	if err != nil {
		return ptyp.TypedefType.Type, err
	}
	runtimeType, err := bi.findType(bi.runtimeTypeTypename())
	if err != nil {
		return ptyp.TypedefType.Type, err
	}
	_type := newVariable("", rtypeAddr, runtimeType, bi, mem)

	mds, err := bi.getModuleData(_type.mem)
	if err != nil {
		return ptyp.TypedefType.Type, err
	}

	typ, _, err := RuntimeTypeToDIE(_type, 0, mds)
	if err != nil {
		return ptyp.TypedefType.Type, err
	}

	return typ, nil
}

func dwarfToRuntimeType(bi *BinaryInfo, mem MemoryReadWriter, typ godwarf.Type) (typeAddr uint64, typeKind uint64, found bool, err error) {
	so := bi.typeToImage(typ)
	rdr := so.DwarfReader()
	rdr.Seek(typ.Common().Offset)
	e, err := rdr.Next()
	if err != nil {
		return 0, 0, false, err
	}
	off, ok := e.Val(godwarf.AttrGoRuntimeType).(uint64)
	if !ok {
		return 0, 0, false, nil
	}

	mds, err := LoadModuleData(bi, mem)
	if err != nil {
		return 0, 0, false, err
	}

	md := bi.imageToModuleData(so, mds)
	if md == nil {
		if so.index > 0 {
			return 0, 0, false, fmt.Errorf("could not find module data for type %s (shared object: %q)", typ, so.Path)
		} else {
			return 0, 0, false, fmt.Errorf("could not find module data for type %s", typ)
		}
	}

	typeAddr = md.types + off

	rtyp, err := bi.findType(bi.runtimeTypeTypename())
	if err != nil {
		return 0, 0, false, err
	}
	_type := newVariable("", typeAddr, rtyp, bi, mem)
	kindv := _type.loadFieldNamed("kind")
	if kindv == nil || kindv.Unreadable != nil || kindv.Kind != reflect.Uint {
		kindv = _type.loadFieldNamed("Kind_")
	}
	if kindv == nil {
		return 0, 0, false, fmt.Errorf("unreadable interface type (no kind field)")
	}
	if kindv.Unreadable != nil || kindv.Kind != reflect.Uint {
		return 0, 0, false, fmt.Errorf("unreadable interface type: %v", kindv.Unreadable)
	}
	typeKind, _ = constant.Uint64Val(kindv.Value)
	return typeAddr, typeKind, true, nil
}
