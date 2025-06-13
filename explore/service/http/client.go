package http

import (
	"bytes"
	"encoding/json"
	"explore/service"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	addr    string
	url     string
	timeout time.Duration
}

func NewClient(addr string) (*Client, error) {
	c := &Client{
		addr:    addr,
		url:     fmt.Sprintf("http://%s", addr),
		timeout: time.Second * 30,
	}

	if !c.IsExploreServer() {
		return nil, fmt.Errorf("%s is not a explore server", c.addr)
	}
	return c, nil
}

func (c *Client) SendExpr(cmdType service.CmdType, args ...string) (string, error) {
	var method, path, expr string
	switch cmdType {
	case service.Set:
		expr = setExpr(args...)
		method = http.MethodPost
		path = "/set"
	case service.List:
		expr = listExpr(args...)
		method = http.MethodGet
		path = "/list"
	case service.Get:
		fallthrough
	default:
		expr = getExpr(args...)
		method = http.MethodGet
		path = "/get"
	}

	resp, err := c.do(&doRequest{
		method: method,
		path:   path,
		expr:   expr,
	})
	if err != nil {
		return "", err
	}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bs), nil
}

func (c *Client) IsExploreServer() bool {
	if c.addr == "" {
		return false
	}

	resp, err := c.do(&doRequest{
		method: http.MethodGet,
		path:   "/explore",
	})
	if err != nil {
		return false
	}

	return resp.StatusCode == http.StatusOK
}

func getExpr(args ...string) string {
	if len(args) < 1 {
		return ""
	}

	return fmt.Sprintf("get %s", args[0])
}

func setExpr(args ...string) string {
	if len(args) < 2 {
		return ""
	}

	return fmt.Sprintf("set %s %s", args[0], args[1])
}

func listExpr(args ...string) string {
	if len(args) < 1 {
		return ""
	}

	return fmt.Sprintf("list %s", args[0])
}

type doRequest struct {
	method string
	path   string
	header http.Header
	expr   string
}

func (c *Client) jsonHeader() http.Header {
	header := http.Header{}
	header.Set("Content-Type", "application/json")

	return header
}

func (c *Client) do(req *doRequest) (resp *http.Response, err error) {
	url := c.url + req.path

	exr := newExpression(req.expr, os.Getpid())
	bs, err := json.Marshal(exr)
	if err != nil {
		return nil, err
	}

	bodyReader := bytes.NewReader(bs)
	request, err := http.NewRequest(req.method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if req.header == nil {
		request.Header = c.jsonHeader()
	} else {
		request.Header = req.header
	}

	http.DefaultClient.Timeout = c.timeout
	return http.DefaultClient.Do(request)
}
