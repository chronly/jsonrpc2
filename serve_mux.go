package jsonrpc2

import (
	"fmt"
	"sync"
)

// ServeMux is an RPC request multiplexer. It matches the method against a list
// of registered handlers and calls the handler whose method matches the request
// directly.
type ServeMux struct {
	mut    sync.RWMutex
	routes map[string]Handler
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux {
	return &ServeMux{routes: make(map[string]Handler)}
}

// Handle registers the handler for a given method. If a handler already exists
// for method, Handle panics.
func (m *ServeMux) Handle(method string, handler Handler) {
	m.mut.Lock()
	defer m.mut.Unlock()

	if _, exist := m.routes[method]; exist {
		panic("method " + method + " already registered")
	}
	m.routes[method] = handler
}

// HandleFunc registers the handler function for the given method.
func (m *ServeMux) HandleFunc(method string, handler func(rw ResponseWriter, r *Request)) {
	m.Handle(method, HandlerFunc(handler))
}

// ServeRPC implements Handler. ServeRPC will find a registered route matching the
// incoming request and invoke it if one exists. When a route wasn't found,
// ErrorMethodNotFound is returned to the caller.
func (m *ServeMux) ServeRPC(w ResponseWriter, req *Request) {
	m.mut.RLock()
	defer m.mut.RUnlock()

	route, ok := m.routes[req.Method]
	if ok {
		route.ServeRPC(w, req)
		return
	}

	if !req.Notification {
		w.WriteError(ErrorMethodNotFound, fmt.Errorf("method %s not found", req.Method))
	}
}
