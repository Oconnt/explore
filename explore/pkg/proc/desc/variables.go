package desc

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

const (
	// strings longer than this will cause slices, arrays and structs to be printed on multiple lines when newlines is enabled
	maxShortStringLen = 7
	// string used for one indentation level (when printing on multiple lines)
	indentString = "\t"
)

type prettyFlags uint8

const (
	prettyTop prettyFlags = 1 << iota
	prettyNewlines
	prettyIncludeType
	prettyShortenType
)

func (flags prettyFlags) top() bool         { return flags&prettyTop != 0 }
func (flags prettyFlags) includeType() bool { return flags&prettyIncludeType != 0 }
func (flags prettyFlags) newlines() bool    { return flags&prettyNewlines != 0 }
func (flags prettyFlags) shortenType() bool { return flags&prettyShortenType != 0 }

func (flags prettyFlags) set(flag prettyFlags, v bool) prettyFlags {
	if v {
		return flags | flag
	} else {
		return flags &^ flag
	}
}

// VariableFlags is the type of the Flags field of Variable.
type VariableFlags uint16

const (
	// VariableEscaped is set for local variables that escaped to the heap
	//
	// The compiler performs escape analysis on local variables, the variables
	// that may outlive the stack frame are allocated on the heap instead and
	// only the address is recorded on the stack. These variables will be
	// marked with this flag.
	VariableEscaped = 1 << iota

	// VariableShadowed is set for local variables that are shadowed by a
	// variable with the same name in another scope
	VariableShadowed

	// VariableConstant means this variable is a constant value
	VariableConstant

	// VariableArgument means this variable is a function argument
	VariableArgument

	// VariableReturnArgument means this variable is a function return value
	VariableReturnArgument

	// VariableFakeAddress means the address of this variable is either fake
	// (i.e. the variable is partially or completely stored in a CPU register
	// and doesn't have a real address) or possibly no longer available (because
	// the variable is the return value of a function call and allocated on a
	// frame that no longer exists)
	VariableFakeAddress

	// VariableCPtr means the variable is a C pointer
	VariableCPtr

	// VariableCPURegister means this variable is a CPU register.
	VariableCPURegister
)

// Variable describes a variable.
type Variable struct {
	// Name of the variable or struct member
	Name string `json:"name"`
	// Address of the variable or struct member
	Addr uint64 `json:"addr"`
	// Only the address field is filled (result of evaluating expressions like &<expr>)
	OnlyAddr bool `json:"onlyAddr"`
	// Go type of the variable
	Type string `json:"type"`
	// Type of the variable after resolving any typedefs
	RealType string `json:"realType"`

	Flags VariableFlags `json:"flags"`

	Kind reflect.Kind `json:"kind"`

	// Strings have their length capped at proc.maxArrayValues, use Len for the real length of a string
	// Function variables will store the name of the function in this field
	Value string `json:"value"`

	// Number of elements in an array or a slice, number of keys for a map, number of struct members for a struct, length of strings, number of captured variables for functions
	Len int64 `json:"len"`
	// Cap value for slices
	Cap int64 `json:"cap"`

	// Array and slice elements, member fields of structs, key/value pairs of maps, value of complex numbers, captured variables of functions.
	// The Name field in this slice will always be the empty string except for structs (when it will be the field name) and for complex numbers (when it will be "real" and "imaginary")
	// For maps each map entry will have to items in this slice, even numbered items will represent map keys and odd numbered items will represent their values
	// This field's length is capped at proc.maxArrayValues for slices and arrays and 2*proc.maxArrayValues for maps, in the circumstances where the cap takes effect len(Children) != Len
	// The other length cap applied to this field is related to maximum recursion depth, when the maximum recursion depth is reached this field is left empty, contrary to the previous one this cap also applies to structs (otherwise structs will always have all their member fields returned)
	Children []Variable `json:"children"`

	// Base address of arrays, Base address of the backing array for slices (0 for nil slices)
	// Base address of the backing byte array for strings
	// address of the struct backing chan and map variables
	// address of the function entry point for function variables (0 for nil function pointers)
	Base uint64 `json:"base"`

	// Unreadable addresses will have this field set
	Unreadable string `json:"unreadable"`

	// LocationExpr describes the location expression of this variable's address
	LocationExpr string
	// DeclLine is the line number of this variable's declaration
	DeclLine int64
}

// SinglelineString returns a representation of v on a single line.
func (v *Variable) SinglelineString() string {
	var buf bytes.Buffer
	v.writeTo(&buf, prettyTop|prettyIncludeType, "", "")
	return buf.String()
}

// SinglelineStringWithShortTypes returns a representation of v on a single line, with types shortened.
func (v *Variable) SinglelineStringWithShortTypes() string {
	var buf bytes.Buffer
	v.writeTo(&buf, prettyTop|prettyIncludeType|prettyShortenType, "", "")
	return buf.String()
}

// SinglelineStringFormatted returns a representation of v on a single line, using the format specified by fmtstr.
func (v *Variable) SinglelineStringFormatted(fmtstr string) string {
	var buf bytes.Buffer
	v.writeTo(&buf, prettyTop|prettyIncludeType, "", fmtstr)
	return buf.String()
}

// MultilineString returns a representation of v on multiple lines.
func (v *Variable) MultilineString(indent, fmtstr string) string {
	var buf bytes.Buffer
	v.writeTo(&buf, prettyTop|prettyNewlines|prettyIncludeType, indent, fmtstr)
	return buf.String()
}

func (v *Variable) writeTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	if v.Unreadable != "" {
		fmt.Fprintf(buf, "(unreadable %s)", v.Unreadable)
		return
	}

	if !flags.top() && v.Addr == 0 && v.Value == "" {
		if flags.includeType() && v.Type != "void" {
			fmt.Fprintf(buf, "%s nil", v.typeStr(flags))
		} else {
			fmt.Fprint(buf, "nil")
		}
		return
	}

	switch v.Kind {
	case reflect.Slice:
		v.writeSliceTo(buf, flags, indent, fmtstr)
	case reflect.Array:
		v.writeArrayTo(buf, flags, indent, fmtstr)
	case reflect.Ptr:
		if v.Type == "" || len(v.Children) == 0 {
			fmt.Fprint(buf, "nil")
		} else if v.Children[0].OnlyAddr && v.Children[0].Addr != 0 {
			v.writePointerTo(buf, flags)
		} else {
			if flags.top() && flags.newlines() && v.Children[0].Addr != 0 {
				v.writePointerTo(buf, flags)
				fmt.Fprint(buf, "\n")
			}
			fmt.Fprint(buf, "*")
			v.Children[0].writeTo(buf, flags.set(prettyTop, false), indent, fmtstr)
		}
	case reflect.UnsafePointer:
		if len(v.Children) == 0 {
			fmt.Fprintf(buf, "unsafe.Pointer(nil)")
		} else {
			fmt.Fprintf(buf, "unsafe.Pointer(%#x)", v.Children[0].Addr)
		}
	case reflect.Chan:
		if flags.newlines() {
			v.writeStructTo(buf, flags, indent, fmtstr)
		} else {
			if len(v.Children) == 0 {
				fmt.Fprintf(buf, "%s nil", v.typeStr(flags))
			} else {
				fmt.Fprintf(buf, "%s %s/%s", v.typeStr(flags), v.Children[0].Value, v.Children[1].Value)
			}
		}
	case reflect.Struct:
		if v.Value != "" {
			fmt.Fprintf(buf, "%s(%s)", v.typeStr(flags), v.Value)
			flags = flags.set(prettyIncludeType, false)
		}
		v.writeStructTo(buf, flags, indent, fmtstr)
	case reflect.Interface:
		if v.Addr == 0 {
			// an escaped interface variable that points to nil, this shouldn't
			// happen in normal code but can happen if the variable is out of scope.
			fmt.Fprintf(buf, "nil")
			return
		}
		if flags.includeType() {
			if v.Children[0].Kind == reflect.Invalid {
				fmt.Fprintf(buf, "%s ", v.typeStr(flags))
				if v.Children[0].Addr == 0 {
					fmt.Fprint(buf, "nil")
					return
				}
			} else {
				fmt.Fprintf(buf, "%s(%s) ", v.typeStr(flags), v.Children[0].Type)
			}
		}
		data := v.Children[0]
		if data.Kind == reflect.Ptr {
			if len(data.Children) == 0 {
				fmt.Fprint(buf, "...")
			} else if data.Children[0].Addr == 0 {
				fmt.Fprint(buf, "nil")
			} else if data.Children[0].OnlyAddr {
				fmt.Fprintf(buf, "0x%x", v.Children[0].Addr)
			} else {
				v.Children[0].writeTo(buf, flags.set(prettyTop, false).set(prettyIncludeType, !flags.includeType()), indent, fmtstr)
			}
		} else if data.OnlyAddr {
			if strings.Contains(v.Type, "/") {
				fmt.Fprintf(buf, "*(*%q)(%#x)", v.typeStr(flags), v.Addr)
			} else {
				fmt.Fprintf(buf, "*(*%s)(%#x)", v.typeStr(flags), v.Addr)
			}
		} else {
			v.Children[0].writeTo(buf, flags.set(prettyTop, false).set(prettyIncludeType, !flags.includeType()), indent, fmtstr)
		}
	case reflect.Map:
		v.writeMapTo(buf, flags, indent, fmtstr)
	case reflect.Func:
		if v.Value == "" {
			fmt.Fprint(buf, "nil")
		} else {
			fmt.Fprintf(buf, "%s", v.Value)
			if flags.newlines() && len(v.Children) > 0 {
				fmt.Fprintf(buf, " {\n")
				for i := range v.Children {
					fmt.Fprintf(buf, "%s%s%s %s = ", indent, indentString, v.Children[i].Name, v.Children[i].typeStr(flags))
					v.Children[i].writeTo(buf, flags.set(prettyTop, false).set(prettyIncludeType, false), indent+indentString, fmtstr)
					fmt.Fprintf(buf, "\n")
				}
				fmt.Fprintf(buf, "%s}", indent)
			}
		}
	default:
		v.writeBasicType(buf, fmtstr)
	}
}

func (v *Variable) typeStr(flags prettyFlags) string {
	if flags.shortenType() {
		return ShortenType(v.Type)
	}

	return v.Type
}

func (v *Variable) writeMapTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	if flags.includeType() {
		fmt.Fprintf(buf, "%s ", v.typeStr(flags))
	}
	if v.Base == 0 && len(v.Children) == 0 {
		fmt.Fprintf(buf, "nil")
		return
	}

	nl := flags.newlines() && (len(v.Children) > 0)

	fmt.Fprint(buf, "[")

	for i := 0; i < len(v.Children); i += 2 {
		key := &v.Children[i]
		value := &v.Children[i+1]

		if nl {
			fmt.Fprintf(buf, "\n%s%s", indent, indentString)
		}

		key.writeTo(buf, 0, indent+indentString, fmtstr)
		fmt.Fprint(buf, ": ")
		value.writeTo(buf, prettyFlags(0).set(prettyNewlines, nl), indent+indentString, fmtstr)
		if i != len(v.Children)-1 || nl {
			fmt.Fprint(buf, ", ")
		}
	}

	if len(v.Children)/2 != int(v.Len) {
		if len(v.Children) != 0 {
			if nl {
				fmt.Fprintf(buf, "\n%s%s", indent, indentString)
			} else {
				fmt.Fprint(buf, ",")
			}
			fmt.Fprintf(buf, "...+%d more", int(v.Len)-(len(v.Children)/2))
		} else {
			fmt.Fprint(buf, "...")
		}
	}

	if nl {
		fmt.Fprintf(buf, "\n%s", indent)
	}
	fmt.Fprint(buf, "]")
}

func (v *Variable) writeSliceTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	if flags.includeType() {
		fmt.Fprintf(buf, "%s len: %d, cap: %d, ", v.typeStr(flags), v.Len, v.Cap)
	}
	if v.Base == 0 && len(v.Children) == 0 {
		fmt.Fprintf(buf, "nil")
		return
	}
	v.writeSliceOrArrayTo(buf, flags, indent, fmtstr)
}

func (v *Variable) writeArrayTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	if flags.includeType() {
		fmt.Fprintf(buf, "%s ", v.typeStr(flags))
	}
	v.writeSliceOrArrayTo(buf, flags, indent, fmtstr)
}

func (v *Variable) writeSliceOrArrayTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	nl := v.shouldNewlineArray(flags.newlines())
	fmt.Fprint(buf, "[")

	for i := range v.Children {
		if nl {
			fmt.Fprintf(buf, "\n%s%s", indent, indentString)
		}
		v.Children[i].writeTo(buf, prettyFlags(0).set(prettyNewlines, nl), indent+indentString, fmtstr)
		if i != len(v.Children)-1 || nl {
			fmt.Fprint(buf, ",")
		}
	}

	if len(v.Children) != int(v.Len) {
		if len(v.Children) != 0 {
			if nl {
				fmt.Fprintf(buf, "\n%s%s", indent, indentString)
			} else {
				fmt.Fprint(buf, ",")
			}
			fmt.Fprintf(buf, "...+%d more", int(v.Len)-len(v.Children))
		} else {
			fmt.Fprint(buf, "...")
		}
	}

	if nl {
		fmt.Fprintf(buf, "\n%s", indent)
	}

	fmt.Fprint(buf, "]")
}

func (v *Variable) recursiveKind() (reflect.Kind, bool) {
	hasptr := false
	var kind reflect.Kind
	for {
		kind = v.Kind
		if kind == reflect.Ptr {
			hasptr = true
			if len(v.Children) == 0 {
				return kind, hasptr
			}
			v = &(v.Children[0])
		} else {
			break
		}
	}
	return kind, hasptr
}

func (v *Variable) shouldNewlineArray(newlines bool) bool {
	if !newlines || len(v.Children) == 0 {
		return false
	}

	kind, hasptr := (&v.Children[0]).recursiveKind()

	switch kind {
	case reflect.Slice, reflect.Array, reflect.Struct, reflect.Map, reflect.Interface:
		return true
	case reflect.String:
		if hasptr {
			return true
		}
		for i := range v.Children {
			if len(v.Children[i].Value) > maxShortStringLen {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (v *Variable) shouldNewlineStruct(newlines bool) bool {
	if !newlines || len(v.Children) == 0 {
		return false
	}

	for i := range v.Children {
		kind, hasptr := (&v.Children[i]).recursiveKind()

		switch kind {
		case reflect.Slice, reflect.Array, reflect.Struct, reflect.Map, reflect.Interface:
			return true
		case reflect.String:
			if hasptr {
				return true
			}
			if len(v.Children[i].Value) > maxShortStringLen {
				return true
			}
		}
	}

	return false
}

func (v *Variable) writeStructTo(buf io.Writer, flags prettyFlags, indent, fmtstr string) {
	if int(v.Len) != len(v.Children) && len(v.Children) == 0 {
		if strings.Contains(v.Type, "/") {
			fmt.Fprintf(buf, "(*%q)(%#x)", v.typeStr(flags), v.Addr)
		} else {
			fmt.Fprintf(buf, "(*%s)(%#x)", v.typeStr(flags), v.Addr)
		}
		return
	}

	if flags.includeType() {
		fmt.Fprintf(buf, "%s ", v.typeStr(flags))
	}

	nl := v.shouldNewlineStruct(flags.newlines())

	fmt.Fprint(buf, "{")

	for i := range v.Children {
		if nl {
			fmt.Fprintf(buf, "\n%s%s", indent, indentString)
		}
		fmt.Fprintf(buf, "%s: ", v.Children[i].Name)
		v.Children[i].writeTo(buf, prettyIncludeType.set(prettyNewlines, nl), indent+indentString, fmtstr)
		if i != len(v.Children)-1 || nl {
			fmt.Fprint(buf, ",")
			if !nl {
				fmt.Fprint(buf, " ")
			}
		}
	}

	if len(v.Children) != int(v.Len) {
		if nl {
			fmt.Fprintf(buf, "\n%s%s", indent, indentString)
		} else {
			fmt.Fprint(buf, ",")
		}
		fmt.Fprintf(buf, "...+%d more", int(v.Len)-len(v.Children))
	}

	fmt.Fprint(buf, "}")
}

func (v *Variable) writePointerTo(buf io.Writer, flags prettyFlags) {
	if strings.Contains(v.Type, "/") {
		fmt.Fprintf(buf, "(%q)(%#x)", v.typeStr(flags), v.Children[0].Addr)
	} else {
		fmt.Fprintf(buf, "(%s)(%#x)", v.typeStr(flags), v.Children[0].Addr)
	}
}

func (v *Variable) writeBasicType(buf io.Writer, fmtstr string) {
	if v.Value == "" && v.Kind != reflect.String {
		fmt.Fprintf(buf, "(unknown %s)", v.Kind)
		return
	}

	switch v.Kind {
	case reflect.Bool:
		if fmtstr == "" {
			buf.Write([]byte(v.Value))
			return
		}
		var b bool = v.Value == "true"
		fmt.Fprintf(buf, fmtstr, b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fmtstr == "" {
			buf.Write([]byte(v.Value))
			return
		}
		n, _ := strconv.ParseInt(ExtractIntValue(v.Value), 10, 64)
		fmt.Fprintf(buf, fmtstr, n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if fmtstr == "" {
			buf.Write([]byte(v.Value))
			return
		}
		n, _ := strconv.ParseUint(ExtractIntValue(v.Value), 10, 64)
		fmt.Fprintf(buf, fmtstr, n)

	case reflect.Float32, reflect.Float64:
		if fmtstr == "" {
			buf.Write([]byte(v.Value))
			return
		}
		x, _ := strconv.ParseFloat(v.Value, 64)
		fmt.Fprintf(buf, fmtstr, x)

	case reflect.Complex64, reflect.Complex128:
		if fmtstr == "" {
			fmt.Fprintf(buf, "(%s + %si)", v.Children[0].Value, v.Children[1].Value)
			return
		}
		real, _ := strconv.ParseFloat(v.Children[0].Value, 64)
		imag, _ := strconv.ParseFloat(v.Children[1].Value, 64)
		var x complex128 = complex(real, imag)
		fmt.Fprintf(buf, fmtstr, x)

	case reflect.String:
		if fmtstr == "" {
			s := v.Value
			if len(s) != int(v.Len) {
				s = fmt.Sprintf("%s...+%d more", s, int(v.Len)-len(s))
			}
			fmt.Fprintf(buf, "%q", s)
			return
		}
		fmt.Fprintf(buf, fmtstr, v.Value)
	}
}

func ExtractIntValue(s string) string {
	if s == "" || s[len(s)-1] != ')' {
		return s
	}
	open := strings.LastIndex(s, "(")
	if open < 0 {
		return s
	}
	return s[open+1 : len(s)-1]
}
