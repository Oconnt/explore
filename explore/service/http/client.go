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

func (c *Client) SendExpr(cmdType service.CmdType, args string) (string, error) {
	var method, path, expr string
	switch cmdType {
	case service.Set:
		expr = setExpr(args)
		method = http.MethodPost
		path = "/set"
	case service.List:
		expr = listExpr(args)
		method = http.MethodGet
		path = "/list"
	case service.Get:
		fallthrough
	default:
		expr = getExpr(args)
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

	respStr, ok := resp.Data.(string)
	if !ok {
		return "", fmt.Errorf("unexpected response type %T", resp.Data)
	}

	return respStr, nil
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
		fmt.Println("client recv err: ", err)
		return false
	}

	return resp.Status == http.StatusOK
}

func getExpr(args string) string {
	return fmt.Sprintf("get %s", args)
}

func setExpr(args string) string {
	return fmt.Sprintf("set %s", args)
}

func listExpr(args string) string {
	return fmt.Sprintf("list %s", args)
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

func (c *Client) do(req *doRequest) (resp *response, err error) {
	url := c.url + req.path

	exr := newExpression(req.expr, os.Getpid())
	bs, err := json.Marshal(exr)
	if err != nil {
		return
	}

	bodyReader := bytes.NewReader(bs)
	r, err := http.NewRequest(req.method, url, bodyReader)
	if err != nil {
		return
	}

	if req.header == nil {
		r.Header = c.jsonHeader()
	} else {
		r.Header = req.header
	}

	http.DefaultClient.Timeout = c.timeout
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return
	}

	bs, err = io.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(bs, &resp)
	return
}
