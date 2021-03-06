package jsonrpc2

import (
	"encoding/json"
	"fmt"
)

// Handler handles an individual RPC call.
type Handler interface {
	// ServeRPC is invoked when an RPC call is received. If a response is needed
	// for a request, null will be sent as the response result. If the request is
	// a notification, ResponseWriter must NOT be used, even for delivering an
	// error.
	//
	// Written responses may not be delivered right away if the request is a batch
	// request.
	ServeRPC(w ResponseWriter, r *Request)
}

type ResponseWriter interface {
	// WriteMessage writes a success response to the client. The value as provided
	// here will be marshaled to json. An error will be returned if the msg could
	// not be marshaled to JSON.
	WriteMessage(msg interface{}) error

	// WriteError writes an error response to the caller.
	WriteError(errorCode int, err error) error
}

type Request struct {
	Method string
	Params json.RawMessage
	Conn   Conn
}

// HandlerFunc implements Handler.
type HandlerFunc func(w ResponseWriter, r *Request)

// ServeRPC implements Handler.
func (f HandlerFunc) ServeRPC(w ResponseWriter, r *Request) {
	f(w, r)
}

// DefaultHandler is the default handler used by a server. It returns
// ErrorMethodNotFound for each RPC.
var DefaultHandler = HandlerFunc(func(w ResponseWriter, r *Request) {
	w.WriteError(ErrorMethodNotFound, fmt.Errorf("method %s not found", r.Method))
})
