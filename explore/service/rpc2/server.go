package rpc2

import "explore/pkg/prowler"

type RPCServer struct {
	prowler *prowler.Prowler
}

func NewRPCServer(prowler *prowler.Prowler) *RPCServer {
	return &RPCServer{
		prowler: prowler,
	}
}

func (r *RPCServer) Run() error {

}

func (r *RPCServer) Get(name string) {

}
