package http

import (
	"encoding/json"
	"explore/pkg/logflags"
	"net/http"
)

type Context struct {
	logger   logflags.Logger
	expr     *Expression
	index    int
	chain    HandlerChain
	request  *request
	response *response
	read     *http.Request
	write    http.ResponseWriter
}

func newContext(logger logflags.Logger, w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		logger: logger,
		read:   r,
		write:  w,
	}
}

func (c *Context) respSuccess(data interface{}) {
	c.resp(http.StatusOK, "", data)
}

func (c *Context) respFailed(code int, message string) {
	c.resp(code, message, nil)
}

func (c *Context) resp(status int, msg string, data interface{}) {
	c.response = &response{
		Status: status,
		Msg:    msg,
		Data:   data,
	}

	bs, err := json.Marshal(c.response)
	if err != nil {
		c.write.WriteHeader(http.StatusInternalServerError)
		c.write.Write([]byte(err.Error()))
		return
	}
	c.write.WriteHeader(status)
	c.write.Write(bs)
}

func (c *Context) Next() {
	c.index++
	if c.index < len(c.chain) {
		handler := c.chain[c.index]
		handler(c)
	}
}
