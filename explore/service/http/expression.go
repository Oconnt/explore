package http

import "strings"

type Expression struct {
	Expr string `json:"expression"`
	Pid  int    `json:"pid"`
}

func newExpression(expr string, pid int) *Expression {
	return &Expression{Expr: expr, Pid: pid}
}

func (e *Expression) resolve() (string, []string) {
	cmds := strings.SplitN(e.Expr, " ", 2)
	return cmds[0], strings.Split(cmds[1], " ")
}
