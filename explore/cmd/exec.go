package cmd

import (
	"explore/pkg/prowler"
	"explore/pkg/terminal"
	"explore/service"
	"explore/service/http"
	"explore/utils"
	"github.com/urfave/cli"
	"log"
	"net"
)

type ExecType int

const (
	Get ExecType = iota
	Set
	List
	Attach
	Conn
)

const (
	defaultAddr = "127.0.0.1:0"
)

type executor struct {
	et      ExecType
	pid     int
	ctx     *cli.Context
	prowler *prowler.Prowler
}

func newExecutor(et ExecType, pid int, ctx *cli.Context) *executor {
	p, _ := prowler.NewProwler(pid)
	return &executor{
		et:      et,
		pid:     pid,
		ctx:     ctx,
		prowler: p,
	}
}

func (e *executor) run() error {
	switch e.et {
	case Get:
		return e.get()
	case Set:
		return e.set()
	case List:
		return e.list()
	case Attach:
		return e.attach()
	case Conn:
		args := e.ctx.Args()
		return e.connect(args.First())
	}

	return nil
}

func exec(et ExecType, pid int, ctx *cli.Context) error {
	ex := newExecutor(et, pid, ctx)
	return ex.run()
}

func (e *executor) get() error {
	args := e.ctx.Args()
	r := rArgs(args)

	v, err := e.prowler.Get(r.name)
	if err != nil {
		return err
	}

	utils.PrintVariable(v)
	return nil
}

func (e *executor) set() error {
	args := e.ctx.Args()
	w := wArgs(args)

	err := e.prowler.Set(w.name, w.value)
	if err != nil {
		return err
	}

	return e.get()
}

func (e *executor) list() error {
	t := e.ctx.Int("type")
	prefixes := e.ctx.StringSlice("prefixes")
	suffixes := e.ctx.StringSlice("suffixes")
	vars := e.prowler.List(prowler.LsType(t), prefixes, suffixes)
	utils.PrintStringLine(vars...)
	return nil
}

func (e *executor) attach() error {
	var server service.Server
	ctx := e.ctx

	listener, err := net.Listen("tcp", defaultAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := ctx.String("srv")
	switch srv {
	case "http":
		server = http.NewServer(listener, e.prowler)
	default:
		server = http.NewServer(listener, e.prowler)
	}

	defer server.Stop()
	if err := server.Run(); err != nil {
		return err
	}

	return e.connect(listener.Addr().String())
}

func (e *executor) connect(addr string) (err error) {
	var client service.Client
	srv := e.ctx.String("srv")
	switch srv {
	case "http":
		fallthrough
	default:
		client, err = http.NewClient(addr)
		if err != nil {
			return
		}
	}

	term := terminal.New(client)
	return term.Run()
}
