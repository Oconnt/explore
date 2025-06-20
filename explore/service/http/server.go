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
	//s.printRequestInfo(r)
	//
	//bs, err := io.ReadAll(r.Body)
	//r.Body.Close()
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusBadRequest)
	//	return
	//}
	//
	//exr := new(Expression)
	//if err = json.Unmarshal(bs, exr); err != nil {
	//	http.Error(w, err.Error(), http.StatusBadRequest)
	//	return
	//}
	//
	//ctx := &Context{
	//	expr:   exr,
	//	method: r.Method,
	//	path:   r.URL.Path,
	//	w:      w,
	//}
	//
	ctx := newContext(s.Logger, w, r)
	p := s.pool.Get().(*processor)
	ctx.chain = httpHandlerChain(p.worker)
	ctx.chain.exec(ctx)
}

//func (s *Server) printRequestInfo(r *http.Request) {
//	if s.ServerImpl.Logger != nil {
//		s.ServerImpl.Logger.Infof("client ip: %s", r.RemoteAddr)
//		s.ServerImpl.Logger.Infof("url: %+v", r.URL)
//		s.ServerImpl.Logger.Infof("method: %s", r.Method)
//		s.ServerImpl.Logger.Infof("headers: %+v", r.Header)
//
//		//bs, _ := io.ReadAll(r.Body)
//		//s.ServerImpl.Logger.Infof("body: %s", string(bs))
//	}
//}
