package http

import (
	"encoding/json"
	"explore/utils"
	"github.com/google/uuid"
	"io"
	"net/http"
)

type Handler func(ctx *Context)

type HandlerChain []Handler

func httpHandlerChain(do Handler) HandlerChain {
	return []Handler{
		parseRequest,
		printRequest,
		parseExpression,
		do,
		printResponse,
	}
}

func (h HandlerChain) exec(ctx *Context) {
	for _, handler := range h {
		handler(ctx)
	}
}

func parseRequest(ctx *Context) {
	if ctx.read != nil {
		r := &request{
			requestID: uuid.New().String(),
			url:       utils.GetFullURL(ctx.read),
			path:      ctx.read.URL.Path,
			method:    ctx.read.Method,
			clientIP:  utils.GetClientIP(ctx.read),
		}

		bs, err := io.ReadAll(ctx.read.Body)
		if err != nil {
			ctx.respFailed(http.StatusBadRequest, err.Error())
			return
		}
		r.body = bs

		ctx.request = r
	}
}

func parseExpression(ctx *Context) {
	req := ctx.request
	if req != nil {
		exr := new(Expression)
		if err := json.Unmarshal(req.body, exr); err != nil {
			ctx.respFailed(http.StatusBadRequest, err.Error())
		}

		ctx.expr = exr
	}
}

func printRequest(ctx *Context) {
	logger := ctx.logger
	req := ctx.request
	if logger != nil && req != nil {
		logger.Info("=========== request info ===========")
		logger.Infof("id: %s", req.requestID)
		logger.Infof("url: %s", req.url)
		logger.Infof("method: %s", req.method)
		logger.Infof("clientIP: %s", req.clientIP)
		logger.Infof("path: %s", req.path)
		logger.Infof("body: %s", string(req.body))
	}
}

func printResponse(ctx *Context) {
	logger := ctx.logger
	res := ctx.response
	if logger != nil && res != nil {
		logger.Info("=========== response info ===========")
		if ctx.request != nil {
			logger.Infof("id: %s", ctx.request.requestID)
		}
		logger.Infof("status: %d", res.Status)
		logger.Infof("msg: %s", res.Msg)
		logger.Infof("data: %+v", res.Data)
	}
}
