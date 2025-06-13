package service

import (
	"net"
)

// Server represents a server for a remote client
// to connect to.
type Server interface {
	Run() error
	Stop() error
}

type ServerImpl struct {
	Listener net.Listener
	StopChan chan struct{}
}

//func (s *ServerImpl) Stop() error {
//	close(s.stopChan)
//	return s.listener.Close()
//}
//
//func (s *ServerImpl) Run() error {
//
//	go func() {
//		defer s.listener.Close()
//		for {
//			_, err := s.listener.Accept()
//			if err != nil {
//				panic(err)
//			}
//		}
//	}()
//
//	return nil
//}
