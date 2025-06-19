package service

type CmdType int

const (
	Get CmdType = iota
	Set
	List
)

type Client interface {
	SendExpr(exprType CmdType, args string) (string, error)
	IsExploreServer() bool
}
