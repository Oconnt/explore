package http

import (
	"context"
	"explore/pkg/prowler"
	"explore/service"
	"github.com/urfave/cli"
	"net"
	"net/http"
	"os"
	"sync"
)

type Server struct {
	service.ServerImpl
	httpServer *http.Server
	pool       sync.Pool
	chain      HandlerChain
}

func NewServer(ctx *cli.Context, listener net.Listener, p *prowler.Prowler) *Server {
	impl := service.ServerImpl{
		Listener: listener,
		StopChan: make(chan struct{}),
	}
	impl.SetupLogger(ctx.Bool("logFlag"), ctx.String("logStr"), ctx.String("logDesc"))

	s := &Server{
		ServerImpl: impl,
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
	ctx := newContext(s.Logger, w, r)
	p := s.pool.Get().(*processor)
	ctx.chain = httpHandlerChain(p.worker)
	ctx.chain.exec(ctx)
}
