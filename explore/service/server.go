package service

import (
	"explore/pkg/logflags"
	"net"
)

// Server represents a server for a remote client
// to connect to.
type Server interface {
	Run() error
	Stop() error
}

type ServerImpl struct {
	Logger   logflags.Logger
	Listener net.Listener
	StopChan chan struct{}
}

func (si *ServerImpl) SetupLogger(flag bool, logStr, logDest string) error {
	err := logflags.Setup(flag, logStr, logDest)
	if err != nil {
		return err
	}

	switch logStr {
	case "http":
		fallthrough
	default:
		si.Logger = logflags.HTTPLogger()
	}

	return nil
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
