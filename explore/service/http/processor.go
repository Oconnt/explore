package http

import (
	"explore/pkg/prowler"
	"explore/utils"
	"fmt"
	"github.com/derekparker/trie"
	"net/http"
	"strings"
)

type Router struct {
	method string
	path   string
	fn     func(ctx *Context)
}

type processor struct {
	prowler *prowler.Prowler
	router  []*Router
	trie    *trie.Trie
}

func (p *processor) route(method, path string) func(ctx *Context) {
	node, found := p.trie.Find(utils.MD5(methodPath(method, path)))
	if found {
		fn := node.Meta().(func(ctx *Context))
		return fn
	}

	return nil
}

func (p *processor) worker(ctx *Context) {
	req := ctx.request
	fn := p.route(req.method, req.path)
	if fn == nil {
		ctx.respFailed(http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}

	fn(ctx)
}

func newProcessor(p *prowler.Prowler) (*processor, error) {
	proc := &processor{
		prowler: p,
	}

	register(proc)
	return proc, nil
}

func register(p *processor) {
	r := []*Router{
		{
			method: http.MethodGet,
			path:   "/explore",
			fn: func(ctx *Context) {
				ctx.respSuccess(nil)
			},
		},
		{
			method: http.MethodGet,
			path:   "/get",
			fn: func(ctx *Context) {
				expr := ctx.expr
				cmd, args := expr.resolve()
				cmdStr := strings.ToLower(cmd)
				if cmdStr != "get" {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid command: %s", cmdStr))
					return
				}

				if len(args) < 1 {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid number of arguments: %d", len(args)))
					return
				}

				name := args[0]
				res, err := p.prowler.Get(name)
				if err != nil {
					ctx.respFailed(http.StatusInternalServerError, err.Error())
					return
				}

				// ctx.w.WriteHeader(http.StatusOK)
				ctx.respSuccess(res.MultilineString("", ""))
			},
		},
		{
			method: http.MethodPost,
			path:   "/set",
			fn: func(ctx *Context) {
				expr := ctx.expr
				cmd, args := expr.resolve()
				cmdStr := strings.ToLower(cmd)
				if cmdStr != "set" {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid command: %s", cmdStr))
					return
				}

				if len(args) < 2 {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid number of arguments: %d", len(args)))
					return
				}

				name, val := args[0], args[1]
				err := p.prowler.Set(name, val)
				if err != nil {
					ctx.respFailed(http.StatusInternalServerError, err.Error())
					return
				}

				res, err := p.prowler.Get(name)
				if err != nil {
					ctx.respFailed(http.StatusInternalServerError, err.Error())
					return
				}

				//ctx.w.WriteHeader(http.StatusOK)
				ctx.respSuccess(res.MultilineString("", ""))
			},
		},
		{
			method: http.MethodGet,
			path:   "/list",
			fn: func(ctx *Context) {
				expr := ctx.expr
				cmd, args := expr.resolve()
				cmdStr := strings.ToLower(cmd)
				if cmdStr != "list" {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid command: %s", cmdStr))
					return
				}

				if len(args) < 1 {
					ctx.respFailed(http.StatusBadRequest, fmt.Sprintf("invalid number of arguments: %d", len(args)))
					return
				}

				ex := args[0]
				ls := p.prowler.ListFuzzy(ex)
				var buf strings.Builder
				for _, elem := range ls {
					line := elem + "\n"
					buf.WriteString(line)
				}

				//ctx.w.WriteHeader(http.StatusOK)
				ctx.respSuccess(buf.String())
			},
		},
	}

	p.router = r

	t := trie.New()
	for _, router := range p.router {
		md5 := utils.MD5(methodPath(router.method, router.path))
		t.Add(md5, router.fn)
	}

	p.trie = t
}

func methodPath(method, path string) string {
	return fmt.Sprintf("%s:%s", method, path)
}
