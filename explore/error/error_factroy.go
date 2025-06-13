package error

import "errors"

var (
	VariableNotFound = errors.New("variable not found")
	ConstantNotFound = errors.New("constant not found")
	FunctionNotFound = errors.New("function not found")
)
