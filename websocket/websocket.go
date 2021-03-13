// Package websocket implements a shim for github.com/gorilla/websocket that
// can be used within github.com/chronly/jsonrpc2.
package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ReadWriter implements io.ReadWriter for a websocket connection, where each
// read and write will
//
// ReadWriter is safe for concurrent reads and concurrent writes.
//
// ReadWriter does not implement Closer, and users must close the underlying
// *websocket.Conn manually.
type ReadWriter struct {
	readMtx  sync.Mutex
	writeMtx sync.Mutex

	conn *websocket.Conn
}

// FromConn convers conn into a ReadWriter.
func FromConn(conn *websocket.Conn) *ReadWriter {
	return &ReadWriter{conn: conn}
}

func (rw *ReadWriter) Read(p []byte) (n int, err error) {
	rw.readMtx.Lock()
	defer rw.readMtx.Unlock()

	_, r, err := rw.conn.NextReader()
	if err != nil {
		return n, err
	}
	return r.Read(p)
}

func (rw *ReadWriter) Write(p []byte) (n int, err error) {
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
