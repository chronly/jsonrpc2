package jsonrpc2

import (
	"fmt"
	"net"
	"sync"

	"go.uber.org/atomic"
)

// Server is a JSON-RPC 2.0 server that can handle multiple conncurrent
// connections. For cases where only one connection is needed, use
// Client instead.
type Server struct {
	// Handler is the handler to invoke when receiving a JSON-RPC request.
	Handler Handler

	// OnClient may be provided to handle new connections.
	OnClient func(c *Client)

	mut       sync.Mutex
	listeners map[*net.Listener]struct{}
	clis      map[*Client]struct{}
	shutDown  atomic.Bool
}

// Serve starts serving connections from a listener. Each new connection
// will be created as a Client and will start processing messages.
//
// lis will be closed when Serve exits.
//
// If s.OnConn is non-nil, it will be invoked for each connection received.
func (s *Server) Serve(lis net.Listener) error {
	lis = &onceCloseListener{Listener: lis}
	defer lis.Close()

	if !s.trackListener(&lis, true) {
		return fmt.Errorf("server closed")
	}
	defer s.trackListener(&lis, false)

	hdlr := s.Handler
	if hdlr == nil {
		hdlr = DefaultHandler
	}

	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		go s.onConn(conn, hdlr)
	}
}

func (s *Server) onConn(conn net.Conn, handler Handler) {
	// Create a conn
	cli := NewClient(conn, handler)
	if s.OnClient != nil {
		go s.OnClient(cli)
	}
	s.trackClient(cli, true)
	defer s.trackClient(cli, false)
}

func (s *Server) trackListener(lis *net.Listener, add bool) bool {
	s.mut.Lock()
	defer s.mut.Unlock()
	if s.listeners == nil {
		s.listeners = make(map[*net.Listener]struct{})
	}
	if add {
		if s.shutDown.Load() {
			return false
		}
		s.listeners[lis] = struct{}{}
	} else {
		delete(s.listeners, lis)
	}
	return true
}

func (s *Server) trackClient(c *Client, add bool) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if s.clis == nil {
		s.clis = make(map[*Client]struct{})
	}
	if add {
		s.clis[c] = struct{}{}
	} else {
		delete(s.clis, c)
	}
}

// Close closes the server. All listeners will be stopped.
func (s *Server) Close() error {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.shutDown.Store(true)

	var firstError error

	if s.listeners != nil {
		for lis := range s.listeners {
			err := (*lis).Close()
			if err != nil && firstError != nil {
				firstError = err
			}
		}
	}

	if s.clis != nil {
		for cli := range s.clis {
			err := cli.Close()
			if err != nil && firstError != nil {
				firstError = err
			}
		}
	}

	return firstError
}

// onceCloseListener allows a listener to be closed more than once and only
// return the first error.
type onceCloseListener struct {
	net.Listener
	closeOnce sync.Once
	closeErr  error
}

func (oc *onceCloseListener) Close() error {
	oc.closeOnce.Do(oc.close)
	return oc.closeErr
}

func (oc *onceCloseListener) close() { oc.closeErr = oc.Listener.Close() }
