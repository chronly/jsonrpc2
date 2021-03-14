package jsonrpc2

import (
	"sync"

	"github.com/gorilla/websocket"
)

// NewWebsocketClient creates a client from a Gorilla websocket. Closing
// the Client will close the underlying websocket.
//
// This function wraps the websocket connection into a io.ReadWriteCloser and
// calls NewClient.
func NewWebsocketClient(conn *websocket.Conn, handler Handler, opts ...ClientOpt) *Client {
	return NewClient(&wsReadWriter{conn: conn}, handler, opts...)
}

type wsReadWriter struct {
	readMtx  sync.Mutex
	writeMtx sync.Mutex

	conn *websocket.Conn
}

func (rw *wsReadWriter) Read(p []byte) (n int, err error) {
	rw.readMtx.Lock()
	defer rw.readMtx.Unlock()

	_, r, err := rw.conn.NextReader()
	if err != nil {
		return n, err
	}
	return r.Read(p)
}

func (rw *wsReadWriter) Write(p []byte) (n int, err error) {
	rw.writeMtx.Lock()
	defer rw.writeMtx.Unlock()

	w, err := rw.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return n, err
	}
	n, err = w.Write(p)
	if err != nil {
		return
	}
	return n, w.Close()
}

func (rw *wsReadWriter) Close() error {
	return rw.conn.Close()
}
