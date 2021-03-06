package jsonrpc2

import (
	"context"
	"encoding/json"
)

// Conn can be used for sending messages. It is implemented by Client.
type Conn interface {
	// Batch starts a new request batch.
	Batch() *Batch

	// Notify sends a notification request to the other side of the connection.
	// It does not wait for a response, and there is no way of knowing if the other
	// side has succesfully processed the event. An error will only be returned for
	// transport-level problems.
	Notify(method string, msg interface{}) error

	// Invoke invokes an RPC on the other side of the connection and waits for a
	// response. An error will be returned for RPC-level and transport-level problems.
	//
	// RPC-level problems will be specified by using Error.
	Invoke(ctx context.Context, method string, msg interface{}) (json.RawMessage, error)
}
