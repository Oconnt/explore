package proc

import (
	"debug/dwarf"
	"explore/pkg/dwarf/op"
	"explore/pkg/logflags"
	"fmt"
	"go/constant"
	"reflect"
	"strings"

	"explore/pkg/dwarf/godwarf"
)

type errCouldNotFindSymbol struct {
	name string
}

func (e *errCouldNotFindSymbol) Error() string {
	return fmt.Sprintf("could not find symbol %s", e.name)
}

// ModuleData counterpart to runtime.moduleData
type ModuleData struct {
	text, etext   uint64
	types, etypes uint64
	typemapVar    *Variable
}

func LoadModuleData(bi *BinaryInfo, mem MemoryReadWriter) ([]ModuleData, error) {
	// +rtype -var firstmoduledata moduledata
	// +rtype -field moduledata.text uintptr
	// +rtype -field moduledata.types uintptr

	var md *Variable
	md, err := findGlobal(bi, mem, "runtime", "firstmoduledata")
	if err != nil {
		return nil, err
	}

	r := []ModuleData{}

	for md.Addr != 0 {
		const (
			typesField   = "types"
			etypesField  = "etypes"
			textField    = "text"
			etextField   = "etext"
			nextField    = "next"
			typemapField = "typemap"
		)
		vars := map[string]*Variable{}

		for _, fieldName := range []string{typesField, etypesField, textField, etextField, nextField, typemapField} {
			var err error
			vars[fieldName], err = md.structMember(fieldName)
			if err != nil {
				return nil, err
			}
		}

		var err error

		touint := func(name string) (ret uint64) {
			if err == nil {
				var n uint64
				n, err = vars[name].asUint()
				ret = n
			}
			return ret
		}

		r = append(r, ModuleData{
			types: touint(typesField), etypes: touint(etypesField),
			text: touint(textField), etext: touint(etextField),
			typemapVar: vars[typemapField],
		})
		if err != nil {
			return nil, err
		}

		md = vars[nextField].maybeDereference()
		if md.Unreadable != nil {
			return nil, md.Unreadable
		}
	}

	return r, nil
}

func findModuleDataForType(mds []ModuleData, typeAddr uint64) *ModuleData {
	for i := range mds {
		if typeAddr >= mds[i].types && typeAddr < mds[i].etypes {
			return &mds[i]
		}
	}
	return nil
}

func findGlobal(bi *BinaryInfo, mem MemoryReadWriter, pkgName, varName string) (*Variable, error) {
	for _, pkgPath := range bi.PackageMap[pkgName] {
		v, err := findGlobalInternal(bi, mem, pkgPath+"."+varName)
		if err != nil || v != nil {
			return v, err
		}
	}
	v, err := findGlobalInternal(bi, mem, pkgName+"."+varName)
	if err != nil || v != nil {
		return v, err
	}
	return nil, &errCouldNotFindSymbol{fmt.Sprintf("%s.%s", pkgName, varName)}
}

func regsReplaceStaticBase(regs op.DwarfRegisters, image *Image) op.DwarfRegisters {
	regs.StaticBase = image.StaticBase
	return regs
}

func findGlobalInternal(bi *BinaryInfo, mem MemoryReadWriter, name string) (*Variable, error) {
	for _, pkgvar := range bi.packageVars {
		if pkgvar.name == name || strings.HasSuffix(pkgvar.name, "/"+name) {
			reader := pkgvar.cu.image.dwarfReader
			reader.Seek(pkgvar.offset)
			entry, err := reader.Next()
			if err != nil {
				return nil, err
			}

			regs := op.DwarfRegisters{StaticBase: bi.Images[0].StaticBase}
			return extractVarInfoFromEntry(bi, pkgvar.cu.image, regsReplaceStaticBase(regs, pkgvar.cu.image), mem, godwarf.EntryToTree(entry), 0)
		}
	}
	for _, fn := range bi.Functions {
		if fn.Name == name || strings.HasSuffix(fn.Name, "/"+name) {
			//TODO(aarzilli): convert function entry into a function type?
			r := newVariable(fn.Name, fn.Entry, &godwarf.FuncType{}, bi, mem)
			r.Value = constant.MakeString(fn.Name)
			r.Base = fn.Entry
			r.loaded = true
			if fn.Entry == 0 {
				r.Unreadable = fmt.Errorf("function %s is inlined", fn.Name)
			}
			return r, nil
		}
	}
	for dwref, ctyp := range bi.consts {
		for _, cval := range ctyp.values {
			if cval.fullName == name || strings.HasSuffix(cval.fullName, "/"+name) {
				t, err := bi.Images[dwref.imageIndex].Type(dwref.offset)
				if err != nil {
					return nil, err
				}
				v := newVariable(name, 0x0, t, bi, mem)
				switch v.Kind {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					v.Value = constant.MakeInt64(cval.value)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
					v.Value = constant.MakeUint64(uint64(cval.value))
				default:
					return nil, fmt.Errorf("unsupported constant kind %v", v.Kind)
				}
				v.Flags |= VariableConstant
				v.loaded = true
				return v, nil
			}
		}
	}
	return nil, nil
}

// Extracts the name and type of a variable from a dwarf entry
// then executes the instructions given in the  DW_AT_location attribute to grab the variable's address
func extractVarInfoFromEntry(bi *BinaryInfo, image *Image, regs op.DwarfRegisters, mem MemoryReadWriter, entry *godwarf.Tree, dictAddr uint64) (*Variable, error) {
	if entry.Tag != dwarf.TagFormalParameter && entry.Tag != dwarf.TagVariable {
		return nil, fmt.Errorf("invalid entry tag, only supports FormalParameter and Variable, got %s", entry.Tag.String())
	}

	n, t, err := readVarEntry(entry, image)
	if err != nil {
		return nil, err
	}

	t, err = resolveParametricType(bi, mem, t, dictAddr)
	if err != nil {
		// Log the error, keep going with t, which will be the shape type
		logflags.DebuggerLogger().Errorf("could not resolve parametric type of %s: %v", n, err)
	}

	addr, pieces, descr, err := bi.Location(entry, dwarf.AttrLocation, regs.PC(), regs, mem)

	v := newVariable(n, uint64(addr), t, bi, mem)
	if pieces != nil {
		v.Flags |= VariableFakeAddress
	}
	v.LocationExpr = descr
	v.DeclLine, _ = entry.Val(dwarf.AttrDeclLine).(int64)
	if err != nil {
		v.Unreadable = err
	}
	return v, nil
}
