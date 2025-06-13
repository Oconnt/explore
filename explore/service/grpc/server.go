package grpc

import (
	"explore/pkg/prowler"
	"explore/service"
	"google.golang.org/grpc"
	"log"
	"net"
)

type Server struct {
	service.ServerImpl
	grpcServer *grpc.Server
}

//func (s *Server) Term() {
//
//}

func NewServer(addr string, pid int) *Server {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	p, err := prowler.NewProwler(pid)
	if err != nil {
		log.Fatalf("failed to create prowler: %v", err)
	}

	s := &Server{
		ServerImpl: service.ServerImpl{
			Listener: lis,
			StopChan: make(chan struct{}),
			Prowler:  p,
		},
		grpcServer: grpc.NewServer(),
	}

	return s
}

func (s *Server) Run() error {
	return s.grpcServer.Serve(s.Listener)
}

func (s *Server) Stop() error {
	s.grpcServer.Stop()
	return nil
}
