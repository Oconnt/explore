package http

import (
	"context"
	"encoding/json"
	"explore/pkg/prowler"
	"explore/service"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
)

type Server struct {
	service.ServerImpl
	httpServer *http.Server
	pool       sync.Pool
}

func NewServer(listener net.Listener, p *prowler.Prowler) *Server {
	s := &Server{
		ServerImpl: service.ServerImpl{
			Listener: listener,
			StopChan: make(chan struct{}),
		},
		pool: sync.Pool{
			New: func() interface{} {
				proc, _ := newProcessor(p)
				return proc
			},
		},
	}

	s.httpServer = &http.Server{
		Handler: s,
	}

	return s
}

func (s *Server) Run() error {
	go func() {
		if err := s.httpServer.Serve(s.Listener); err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	exr := new(Expression)
	if err = json.Unmarshal(bs, exr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := &Context{
		expr:   exr,
		method: r.Method,
		path:   r.URL.Path,
		w:      w,
	}

	p := s.pool.Get().(*processor)
	p.worker(ctx)
}
