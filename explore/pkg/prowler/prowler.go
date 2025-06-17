package prowler

import (
	"bytes"
	"debug/dwarf"
	"encoding/binary"
	"encoding/json"
	e "explore/error"
	"explore/utils"
	"fmt"
	"github.com/derekparker/trie"
	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	"github.com/go-delve/delve/pkg/proc"
	"github.com/go-delve/delve/pkg/proc/linutil"
	"github.com/go-delve/delve/service/api"
	cst "go/constant"
	"math"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type LsType int

const (
	Vac LsType = iota
	Variable
	Constant
)

var (
	loadFullValue = proc.LoadConfig{FollowPointers: true, MaxVariableRecurse: 1, MaxStringLen: 64, MaxArrayValues: 64, MaxStructFields: -1, MaxMapBuckets: 64}
)

type Prowler struct {
	pid                  int
	bi                   *proc.BinaryInfo
	DebugInfoDirectories []string
	vars                 map[string]*GlobalVar
	constants            map[string]*GlobalConst
	functions            map[string]*proc.Function
	trie                 *trie.Trie
	mu                   sync.Mutex
}

type GlobalVar struct {
	proc.PackageVar
	ty *godwarf.Type
}

type GlobalConst struct {
	name      string
	fullName  string
	value     int64
	singleBit bool
	imgIndex  int
	offset    dwarf.Offset
	ty        *godwarf.Type
}

func (v *GlobalVar) Type() *godwarf.Type {
	if v.ty == nil {
		img := v.Cu.GetImage()
		reader := img.DwarfReader()
		reader.Seek(v.Offset)

		en, _ := reader.Next()
		offset := en.Val(dwarf.AttrType).(dwarf.Offset)
		ty, _ := img.Type(offset)
		v.ty = &ty
	}

	return v.ty
}

func NewProwler(pid int) (*Prowler, error) {
	p := &Prowler{
		pid:                  pid,
		bi:                   proc.NewBinaryInfo(runtime.GOOS, runtime.GOARCH),
		DebugInfoDirectories: []string{"/usr/lib/debug/.build-id"},
		vars:                 make(map[string]*GlobalVar),
		constants:            make(map[string]*GlobalConst),
		functions:            make(map[string]*proc.Function),
	}

	path, entry, di, err := p.LoadParam()
	if err != nil {
		return nil, err
	}

	err = p.bi.LoadBinaryInfo(path, entry, di)
	if err != nil {
		return nil, err
	}

	// 建立索引树
	t := trie.New()

	vs := p.bi.Vars()
	for _, v := range vs {
		gv := &GlobalVar{
			PackageVar: v,
		}
		gv.ty = gv.Type()
		p.vars[v.Name] = gv

		t.Add(v.Name, gv)
	}

	cs := p.bi.Constant()
	for _, cv := range cs {
		if cv.Name == "" {
			continue
		}
		c := &GlobalConst{
			name:      cv.Name,
			fullName:  cv.FullName,
			value:     cv.Value,
			singleBit: cv.SingleBit,
			imgIndex:  cv.ImageIndex,
			offset:    cv.Offset,
		}
		p.constants[c.name] = c

		t.Add(cv.Name, c)
	}

	functions := p.bi.LookupFunc()
	for name, fs := range functions {
		if len(fs) == 1 {
			f := fs[0]
			p.functions[name] = f
			t.Add(f.Name, f)
		} else {
			//fmt.Printf("%s has many func %+v\n", name, fs)
		}
	}

	p.trie = t

	return p, nil
}

func (p *Prowler) Get(name string) (*api.Variable, error) {
	node, found := p.trie.Find(name)
	if !found {
		return nil, fmt.Errorf("%s not found in process", name)
	}

	meta := node.Meta()

	var v *proc.Variable
	switch meta.(type) {
	case *GlobalVar:
		variable, err := p.getVariable(name)
		if err != nil {
			return nil, err
		}
		v = variable
	case *GlobalConst:
		constant, err := p.getConstant(name)
		if err != nil {
			return nil, err
		}
		v = constant
	case *proc.Function:
		function, err := p.getFunction(name)
		if err != nil {
			return nil, err
		}
		v = function
	}

	return p.ToPrintVar(v), nil
}

func (p *Prowler) getVariable(name string) (*proc.Variable, error) {
	pkgVar, ok := p.vars[name]
	if !ok {
		return nil, e.VariableNotFound
	}

	addr := pkgVar.Addr

	return p.ToVar(name, addr)
}

func (p *Prowler) getConstant(name string) (*proc.Variable, error) {
	cons, ok := p.constants[name]
	if !ok {
		return nil, e.ConstantNotFound
	}

	img := p.bi.Images[cons.imgIndex]
	t, err := img.Type(cons.offset)
	if err != nil {
		return nil, err
	}

	var v cst.Value
	switch t.(type) {
	case *godwarf.UintType:
		v = cst.MakeUint64(uint64(cons.value))
	case *godwarf.IntType:
		v = cst.MakeInt64(cons.value)
	case *godwarf.BoolType:
		v = cst.MakeBool(cons.value != 0)
	case *godwarf.FloatType:
		v = cst.MakeFloat64(math.Float64frombits(uint64(cons.value)))
	default:
		v = cst.Make(cons.value)
	}

	return proc.NewConstant(name, v, p), nil
}

func (p *Prowler) getFunction(name string) (*proc.Variable, error) {
	fn, ok := p.functions[name]
	if !ok {
		return nil, e.FunctionNotFound
	}

	v := proc.NewVariable(fn.Name, fn.Entry, &godwarf.FuncType{}, p.bi, p)
	v.Value = cst.MakeString(fn.Name)
	//text, err := p.loadFunction(fn)
	//if err != nil {
	//	return nil, err
	//}
	//
	//for _, t := range text {
	//	tx := t.Text(proc.GoFlavour, p.bi)
	//	fmt.Println("tx: ", tx)
	//}
	//filename, line, _ := p.bi.PCToLine(fn.Entry)
	//fmt.Printf("file: %s, line: %d\n", filename, line)
	//fmt.Printf("function: %v\n", p.bi.Sources)

	return v, nil
}

func (p *Prowler) Set(name string, value string) error {
	src, err := p.getVariable(name)
	if err != nil {
		return err
	}

	val, err := p.Expression(value, src.RealType)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.set(src, val)
}

func (p *Prowler) set(src *proc.Variable, val interface{}) error {
	switch src.Kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fallthrough
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fallthrough
	case reflect.Bool:
		v := cst.Make(val)
		src.Value = v
	case reflect.Float32, reflect.Float64:
		v := cst.MakeFloat64(val.(float64))
		src.Value = v
	case reflect.String:
		ss := val.(string)
		ptr, err := findFreeMemory(p.pid, uint64(len(ss)))
		if err != nil {
			return err
		}

		// 将值写入空闲内存
		if _, err = p.WriteMemory(ptr, []byte(ss)); err != nil {
			return err
		}

		src.Base = ptr
		src.Len = int64(len(ss))
	case reflect.Ptr:
		return p.set(src.Elem(), val)
	case reflect.Struct:
		realType := src.RealType.(*godwarf.StructType)
		fields := realType.Field
		fieldMap := make(map[string]*godwarf.StructField)
		for _, field := range fields {
			fieldMap[field.Name] = field
		}

		structJson, ok := val.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid struct type: %T", val)
		}

		for k, v := range structJson {
			field, found := fieldMap[k]
			if !found {
				return fmt.Errorf("unknown field: %s", k)
			}

			addr := src.Addr + uint64(field.ByteOffset)
			fieldVar := proc.NewVariable("", addr, godwarf.ResolveTypedef(field.Type), p.bi, p)
			err := p.loadValue(fieldVar)
			if err != nil {
				return err
			}

			if err = p.set(fieldVar, v); err != nil {
				return err
			}
		}

		return nil
	case reflect.Chan:
		// If it is to modify chan, only modify the underlying buf
		var bufVar *proc.Variable
		children := src.Children
		for _, child := range children {
			if child.Name == "dataqsiz" && child.Value == cst.MakeUint64(0) {
				return fmt.Errorf("cannot support synchronous channel modification")
			}

			if child.Name == "buf" {
				bufVar = &child
			}
		}

		if bufVar != nil {
			return p.set(bufVar, val)
		}

		return fmt.Errorf("src variable not found buf: %+v\n", src)
	case reflect.Map:
		valMap := val.(map[interface{}]interface{})
		cVars := src.Children

		var key, value interface{}
		addKV := make(map[interface{}]interface{})
		for i, cv := range cVars {
			tv := cv.Value.String()
			if cv.Kind == reflect.String {
				utv, err := strconv.Unquote(tv)
				if err != nil {
					return err
				}
				tv = utv
			}

			if i%2 == 0 {
				// key process
				ek, err := p.Expression(tv, cv.RealType)
				if err != nil {
					return err
				}
				key = ek

				if v, ok := valMap[key]; ok {
					// modify the value of the key
					value = v
				} else {
					// add key value pairs
					addKV[key] = v
				}
			} else {
				// value process
				if value != nil {
					if err := p.set(&cv, value); err != nil {
						return err
					}
					value = nil
				}
			}
		}

		return nil
	case reflect.Slice:
		valSli := val.([]interface{})
		// 计算内存
		l := uintptr(len(valSli)) * reflect.TypeOf(valSli).Elem().Size()
		ptr, err := findFreeMemory(p.pid, uint64(l))
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		for i, v := range valSli {
			elem := reflect.ValueOf(v)
			typ := elem.Type()

			switch typ.Kind() {
			case reflect.String:
				elemLen := elem.Len()
				elemStr := elem.String()
				dataPtr, err := findFreeMemory(p.pid, uint64(elemLen))
				if err != nil {
					return err
				}

				// 将值写入空闲内存
				if _, err = p.WriteMemory(dataPtr, []byte(elemStr)); err != nil {
					return err
				}

				sliCurAddr := ptr + uint64(uintptr(i)*elem.Type().Size())
				err = p.writeString(sliCurAddr, uint64(elemLen), dataPtr)
				if err != nil {
					return err
				}
			default:
				if err = binary.Write(buf, binary.LittleEndian, elem.Interface()); err != nil {
					return fmt.Errorf("serialize error at index %d: %v", i, err)
				}

			}

		}

		if buf.Len() != 0 {
			// 将值写入空闲内存
			if _, err = p.WriteMemory(ptr, buf.Bytes()); err != nil {
				return err
			}
		}

		src.Base = ptr
		src.Len = int64(len(valSli))
		src.Cap = int64(cap(valSli))
	case reflect.Array:
		valSli := val.([]interface{})
		addr := src.Addr
		buf := new(bytes.Buffer)
		for i, v := range valSli {
			elem := reflect.ValueOf(v)
			typ := elem.Type()

			switch typ.Kind() {
			case reflect.String:
				elemLen := elem.Len()
				elemStr := elem.String()
				dataPtr, err := findFreeMemory(p.pid, uint64(elemLen))
				if err != nil {
					return err
				}

				// 将值写入空闲内存
				if _, err = p.WriteMemory(dataPtr, []byte(elemStr)); err != nil {
					return err
				}

				sliCurAddr := addr + uint64(uintptr(i)*elem.Type().Size())
				err = p.writeString(sliCurAddr, uint64(elemLen), dataPtr)
				if err != nil {
					return err
				}
			default:
				if err := binary.Write(buf, binary.LittleEndian, elem.Interface()); err != nil {
					return fmt.Errorf("serialize error at index %d: %v", i, err)
				}

			}

		}

		if buf.Len() != 0 {
			// 将值写入空闲内存
			if _, err := p.WriteMemory(addr, buf.Bytes()); err != nil {
				return err
			}
		}

		return nil
	}

	dst := proc.NewVariable(src.Name, src.Addr, src.DwarfType, p.bi, p)
	return proc.SetValue(dst, src, "")
}

func (p *Prowler) writeString(addr, len, base uint64) error {
	if err := p.writePointer(addr, base); err != nil {
		return err
	}

	return p.writePointer(addr+uint64(p.bi.Arch.PtrSize()), len)
}

func (p *Prowler) writePointer(addr, val uint64) error {
	ptrbuf := make([]byte, p.bi.Arch.PtrSize())

	// TODO: use target architecture endianness instead of LittleEndian
	switch len(ptrbuf) {
	case 4:
		binary.LittleEndian.PutUint32(ptrbuf, uint32(val))
	case 8:
		binary.LittleEndian.PutUint64(ptrbuf, val)
	default:
		panic(fmt.Errorf("unsupported pointer size %d", len(ptrbuf)))
	}
	_, err := p.WriteMemory(addr, ptrbuf)
	return err
}

func (p *Prowler) ListFuzzy(expr string) []string {
	return p.trie.FuzzySearch(expr)
}

func (p *Prowler) List(t LsType, prefixes, suffixes []string) []string {
	switch t {
	case Vac:
		return append(p.ListVariables(prefixes, suffixes), p.ListConstants(prefixes, suffixes)...)
	case Variable:
		return p.ListVariables(prefixes, suffixes)
	case Constant:
		return p.ListConstants(prefixes, suffixes)
	default:
		return nil
	}
}

func (p *Prowler) ListVariables(prefixes, suffixes []string) []string {
	all := len(prefixes) == 0 && len(suffixes) == 0

	var variables []string
	for name := range p.vars {
		if all || utils.PrefixIn(name, prefixes) || utils.SuffixIn(name, suffixes) {
			variables = append(variables, name)
		}

	}

	return variables
}

func (p *Prowler) ListConstants(prefixes, suffixes []string) []string {
	all := len(prefixes) == 0 && len(suffixes) == 0

	var constants []string
	for name := range p.constants {
		if all || utils.PrefixIn(name, prefixes) || utils.SuffixIn(name, suffixes) {
			constants = append(constants, name)
		}

	}

	return constants
}

func (p *Prowler) Expression(expr string, realType godwarf.Type) (interface{}, error) {
	switch realType.(type) {
	case *godwarf.BoolType:
		return strconv.ParseBool(expr)
	case *godwarf.IntType:
		return strconv.ParseInt(expr, 10, 64)
	case *godwarf.UintType:
		return strconv.ParseUint(expr, 10, 64)
	case *godwarf.FloatType:
		return strconv.ParseFloat(expr, 64)
	case *godwarf.StringType:
		return expr, nil
	case *godwarf.PtrType:
		ptrType := realType.(*godwarf.PtrType)
		return p.Expression(expr, godwarf.ResolveTypedef(ptrType.Type))
	case *godwarf.StructType:
		var mp map[string]interface{}
		if err := json.Unmarshal([]byte(expr), &mp); err != nil {
			return nil, err
		}

		fieldMap := make(map[string]*godwarf.StructField)
		if err := parseStruct(fieldMap, realType); err != nil {
			return nil, err
		}

		res := make(map[string]interface{})
		for name, val := range mp {
			field, ok := fieldMap[name]
			if !ok {
				return nil, fmt.Errorf("unknown field: %s", name)
			}

			var vs string
			js, ok := val.(map[string]interface{})
			if ok {
				b, err := json.Marshal(js)
				if err != nil {
					return nil, err
				}
				vs = string(b)
			} else {
				vs = fmt.Sprintf("%v", val)
			}

			realVal, err := p.Expression(vs, godwarf.ResolveTypedef(field.Type))
			if err != nil {
				return nil, err
			}
			res[name] = realVal
		}

		return res, nil
	case *godwarf.ChanType:
		if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
			var chBuf []interface{}
			chanType := realType.(*godwarf.ChanType)
			elemType := chanType.ElemType
			expr = expr[1 : len(expr)-1]
			elems := strings.Split(expr, ",")

			for _, rawElem := range elems {
				elem, err := p.Expression(strings.TrimSpace(rawElem), elemType)
				if err != nil {
					return nil, err
				}
				chBuf = append(chBuf, elem)
			}

			return chBuf, nil
		}

		return nil, fmt.Errorf("cannot parse expression %q, chan must be wrapped by []", expr)
	case *godwarf.MapType:
		var mp map[string]interface{}
		if err := json.Unmarshal([]byte(expr), &mp); err != nil {
			return nil, err
		}

		mapType := realType.(*godwarf.MapType)
		keyType := mapType.KeyType
		elemType := mapType.ElemType

		res := make(map[interface{}]interface{})
		for k, v := range mp {
			key, err := p.Expression(k, godwarf.ResolveTypedef(keyType))
			if err != nil {
				return nil, fmt.Errorf("key format error, err: %v, elem: %s, type: %v", err, key, keyType)
			}

			var vs string
			js, ok := v.(map[string]interface{})
			if ok {
				b, err := json.Marshal(js)
				if err != nil {
					return nil, err
				}
				vs = string(b)
			} else {
				vs = fmt.Sprintf("%v", v)
			}

			elem, err := p.Expression(vs, godwarf.ResolveTypedef(elemType))
			if err != nil {
				return nil, fmt.Errorf("elem format error, err: %v, elem: %v, type: %v", err, elem, elemType)
			}

			res[key] = elem
		}

		return res, nil
	case *godwarf.SliceType:
		if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
			var items []interface{}
			elemType := realType.(*godwarf.SliceType).ElemType
			expr = expr[1 : len(expr)-1]
			if len(expr) == 0 {
				return items, nil
			}

			elems := strings.Split(expr, ",")
			for _, elem := range elems {
				exp, err := p.Expression(strings.TrimSpace(elem), elemType)
				if err != nil {
					return nil, fmt.Errorf("element format error, elem: %s, type: %v", elem, elemType)
				}
				items = append(items, exp)
			}

			return items, nil
		}

		return nil, fmt.Errorf("cannot parse expression %q, sli must be wrapped by []", expr)
	case *godwarf.ArrayType:
		if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
			var items []interface{}
			arrType := realType.(*godwarf.ArrayType)
			count := arrType.Count
			elemType := arrType.Type
			expr = expr[1 : len(expr)-1]
			if len(expr) == 0 {
				return items, nil
			}

			elems := strings.Split(expr, ",")
			if len(elems) > int(count) {
				return nil, fmt.Errorf("expected length is %d, actual length is %d, array length is not expandable, write failed", count, len(elems))
			}

			for _, elem := range elems {
				exp, err := p.Expression(strings.TrimSpace(elem), elemType)
				if err != nil {
					return nil, fmt.Errorf("element format error, elem: %s, type: %v", elem, elemType)
				}
				items = append(items, exp)
			}

			return items, nil
		}

		return nil, fmt.Errorf("cannot parse expression %q, arr must be wrapped by []", expr)
	default:
		return nil, fmt.Errorf("conversion not implemented for type: %s", realType.String())
	}
}

func parseStruct(m map[string]*godwarf.StructField, realType godwarf.Type) error {
	t, ok := realType.(*godwarf.StructType)
	if !ok {
		return fmt.Errorf("expected *godwarf.StructType, got %T", realType)
	}

	fields := t.Field
	for _, field := range fields {
		m[field.Name] = field
	}

	return nil
}

func (p *Prowler) uintToBytes(val uint64) []byte {
	ptrBuf := make([]byte, p.bi.Arch.PtrSize())

	switch len(ptrBuf) {
	case 4:
		binary.LittleEndian.PutUint32(ptrBuf, uint32(val))
	case 8:
		binary.LittleEndian.PutUint64(ptrBuf, val)
	default:
		panic(fmt.Errorf("unsupported pointer size %d", len(ptrBuf)))
	}

	return ptrBuf
}

func (p *Prowler) ReadMemory(bs []byte, addr uint64) (int, error) {
	return readMemory(p.pid, bs, uintptr(addr))
}

func (p *Prowler) WriteMemory(addr uint64, bs []byte) (int, error) {
	return writeMemory(p.pid, bs, uintptr(addr))
}

func (p *Prowler) ToVar(name string, addr uint64) (*proc.Variable, error) {
	vv, ok := p.vars[name]
	if !ok {
		return nil, fmt.Errorf("variable %q not found", name)
	}

	v := proc.NewVariable(name, addr, *vv.ty, p.bi, p)
	err := p.loadValue(v)
	if err != nil {
		return nil, err
	}

	fmt.Printf("v: %+v\n", v)

	return v, nil
}

func (p *Prowler) LoadParam() (path string, entry uint64, debugInfoDirectories []string, err error) {
	path = utils.FindExecutable("", p.pid)
	entry, err = p.EntryPoint()
	debugInfoDirectories = p.DebugInfoDirectories
	return
}

func (p *Prowler) EntryPoint() (uint64, error) {
	auxvbuf, err := os.ReadFile(fmt.Sprintf("/proc/%d/auxv", p.pid))
	if err != nil {
		return 0, fmt.Errorf("could not read auxiliary vector: %v", err)
	}

	return linutil.EntryPointFromAuxv(auxvbuf, p.bi.Arch.PtrSize()), nil
}

func (p *Prowler) ToPrintVar(v *proc.Variable) *api.Variable {
	if v == nil {
		return nil
	}

	vv := &api.Variable{
		Name:     v.Name,
		Addr:     v.Addr,
		OnlyAddr: v.OnlyAddr,
		Type:     v.TypeString(),
		Kind:     v.Kind,
		Len:      v.Len,
		Cap:      v.Cap,
		Flags:    api.VariableFlags(v.Flags),
		DeclLine: v.DeclLine,
	}

	if v.RealType != nil {
		vv.RealType = v.RealType.String()
	}

	if v.Value != nil {
		val := v.Value.String()
		if v.TypeString() == "string" {
			val, _ = strconv.Unquote(v.Value.ExactString())
		}

		vv.Value = val
	}

	if v.Children != nil {
		for _, child := range v.Children {
			vv.Children = append(vv.Children, *p.ToPrintVar(&child))
		}
	}

	return vv
}

func (p *Prowler) loadValue(v *proc.Variable) error {
	v.LoadValue(loadFullValue)

	return v.Unreadable
}

func (p *Prowler) disassemble(fn *proc.Function) ([]proc.AsmInstruction, error) {
	bi := p.bi
	startAddr := fn.Entry
	endAddr := fn.End
	mem := make([]byte, int(endAddr-startAddr))
	_, err := p.ReadMemory(mem, startAddr)
	if err != nil {
		return nil, err
	}

	pc := startAddr
	r := make([]proc.AsmInstruction, 0, len(mem)/bi.Arch.MaxInstructionLength())
	asmDecode := bi.Arch.GetAsmDecodeFn()
	for len(mem) > 0 {
		file, line, f := bi.PCToLine(pc)
		var inst proc.AsmInstruction
		inst.Loc = proc.Location{PC: pc, File: file, Line: line, Fn: f}
		err = asmDecode(&inst, mem, nil, p, bi)
		if err != nil {
			return nil, err
		}

		r = append(r, inst)
		pc += uint64(inst.Size)
		mem = mem[inst.Size:]
	}

	return r, nil
}
