package jsonrpc2

import (
	"fmt"
	"sync"
)

// Router is a very simple RPC request router.
type Router struct {
	mut    sync.RWMutex
	routes map[string]Handler
}

// RegisterRoute calls a Handler whenever the specific RPC method
// is received.
func (r *Router) RegisterRoute(method string, handler Handler) {
	r.mut.Lock()
	defer r.mut.Unlock()

	if r.routes == nil {
		r.routes = make(map[string]Handler)
	}
	r.routes[method] = handler
}

// ServeRPC implements Handler. ServeRPC will find a registered route matching the
// incoming request and invoke it if one exists. When a route wasn't found,
// ErrorMethodNotFound is returned to the caller.
func (r *Router) ServeRPC(w ResponseWriter, req *Request) {
	r.mut.RLock()
	defer r.mut.RUnlock()

	route, ok := r.routes[req.Method]
	if ok {
		route.ServeRPC(w, req)
		return
	}

	if !req.Notification {
		w.WriteError(ErrorMethodNotFound, fmt.Errorf("method %s not found", req.Method))
	}
}
