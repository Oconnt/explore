package http

type request struct {
	requestID string
	method    string
	url       string
	body      []byte
	clientIP  string
	path      string
}
