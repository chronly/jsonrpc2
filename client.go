package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"

	"go.uber.org/atomic"
)

type Client struct {
	tx *transport

	// listeners are in-flight messages waiting for a response.
	// listeners is a map of *string to a chan of transport.
	// listeners must NEVER close the channel, but rather must delete their
	// entry from the map and let Go's GC clean up the channel.
	listeners sync.Map

	nextID  *atomic.Int64
	handler Handler
}

// Dial dials to
func Dial(target string) (*Client, error) {
	var d net.Dialer
	nc, err := d.Dial("tcp", target)
	if err != nil {
		return nil, fmt.Errorf("failed diling to server: %w", err)
	}

	cli := &Client{
		tx:      newTransport(nc),
		handler: DefaultHandler,
		nextID:  atomic.NewInt64(0),
	}
	go cli.processMessages()
	return cli, nil
}

// Close closes the underlying transport.
func (c *Client) Close() error {
	return c.tx.c.Close()
}

// processMessages runs in the background and handles incoming messages from
// the server.
func (c *Client) processMessages() {
	for {
		batch, err := c.tx.ReadMessage()
		if err != nil {
			return
		}

		var resp txMessage
		resp.Batched = batch.Batched

	Objects:
		for _, msg := range batch.Objects {
			switch {
			case msg.Request != nil:
				r := c.handleRequest(msg.Request)
				if r != nil {
					resp.Objects = append(resp.Objects, &txObject{Response: r})
				}
			case msg.Response != nil:
				// If the response ID wasn't set, then it's a generic error.
				if msg.Response.ID == nil {
					// TODO(rfratto): log error / increment metric
					continue Objects
				}

				lis, ok := c.listeners.Load(convertID(msg.Response.ID))
				if !ok {
					// The listener either never existed or went away.
					// TODO(rfratto): log warning / increment metric
					continue Objects
				}

				lis.(chan *txObject) <- msg
			}
		}

		if len(resp.Objects) > 0 {
			if err := c.tx.SendMessage(resp); err != nil {
				// TODO(rfratto): log error? increment metric?
			}
		}
	}
}

func convertID(in *string) int64 {
	if in == nil {
		return -1
	}
	res, _ := strconv.ParseInt(*in, 10, 64)
	return res
}

// handleRequest handles an individual request.
func (c *Client) handleRequest(req *txRequest) *txResponse {
	ww := &responseWriter{
		notification: req.Notification,
		resp:         &txResponse{ID: req.ID},
		set:          atomic.NewBool(false),
	}
	c.handler.ServeRPC(ww, &Request{
		Method: req.Method,
		Params: req.Params,
		Conn:   c,
	})

	if ww.resp.Result == nil {
		ww.resp.Result = []byte{}
	}
	return ww.resp
}

/*
type ResponseWriter interface {
	// WriteMessage writes a success response to the client. The value as provided
	// here will be marshaled to json. An error will be returned if the msg could
	// not be marshaled to JSON.
	WriteMessage(msg interface{}) error

	// WriteError writes an error response to the caller.
	WriteError(errorCode int, err error) error
}
*/

type responseWriter struct {
	notification bool
	resp         *txResponse
	set          *atomic.Bool
}

func (w *responseWriter) WriteMessage(msg interface{}) error {
	if w.notification {
		return fmt.Errorf("cannot write message for notification")
	}
	if !w.set.CAS(false, true) {
		return fmt.Errorf("response already set")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	w.resp.Result = json.RawMessage(body)
	return nil
}

func (w *responseWriter) WriteError(errCode int, err error) error {
	if w.notification {
		return fmt.Errorf("cannot write message for notification")
	}
	if !w.set.CAS(false, true) {
		return fmt.Errorf("response already set")
	}

	w.resp.Error = &Error{
		Code:    errCode,
		Message: err.Error(),
	}
	return nil
}

// Batch creates a new request batch.
func (c *Client) Batch() *Batch {
	return &Batch{cli: c}
}

// Notify sends a notification request to the other side of the
// connection. It does not wait for a response, and there is no way of knowing
// if the other side succesfully handled the notification. An error will be
// returned for transport-level problems.
func (c *Client) Notify(method string, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.tx.SendMessage(txMessage{
		Batched: false,
		Objects: []*txObject{{
			Request: &txRequest{
				Notification: true,
				Method:       method,
				Params:       body,
			},
		}},
	})
}

// Invoke invokes an RPC on the other side of the connection and waits
// for a repsonse. Error will be set for RPC-level and transport-level
// problems.
//
// RPC-level errors will be set to the Error object.
func (c *Client) Invoke(ctx context.Context, method string, msg interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	var (
		msgID   = c.nextID.Inc()
		msgText = strconv.FormatInt(msgID, 10)

		respCh = make(chan *txObject, 1)
	)

	c.listeners.Store(msgID, respCh)
	defer c.listeners.Delete(msgID)

	err = c.tx.SendMessage(txMessage{
		Batched: false,
		Objects: []*txObject{{
			Request: &txRequest{
				Notification: false,
				ID:           &msgText,
				Method:       method,
				Params:       body,
			},
		}},
	})
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Response == nil {
			return nil, fmt.Errorf("unexpected message: no response body")
		}
		if resp.Response.Error != nil {
			return nil, *resp.Response.Error
		}
		return resp.Response.Result, nil
	}
}

// Batch is a batch of messages to send to a client. It must be committed with
// Commit. A Batch can be created through the Batch method on a Client.
type Batch struct {
	cli *Client
	msg txMessage

	watchers sync.Map
}

// Notify adds a notification request to the batch.
func (b *Batch) Notify(method string, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	b.msg.Objects = append(b.msg.Objects, &txObject{
		Request: &txRequest{
			Notification: true,
			Method:       method,
			Params:       body,
		},
	})

	return nil
}

// Invoke queues an RPC to invoke. The returned *json.RawMessage will be empty until
// the batch is commited.
func (b *Batch) Invoke(method string, msg interface{}) (*json.RawMessage, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	var (
		msgID   = b.cli.nextID.Inc()
		msgText = strconv.FormatInt(msgID, 10)

		result json.RawMessage
		respCh = make(chan *txObject, 1)
	)

	b.watchers.Store(msgID, &result)
	b.cli.listeners.Store(msgID, respCh)

	b.msg.Objects = append(b.msg.Objects, &txObject{
		Request: &txRequest{
			Notification: false,
			ID:           &msgText,
			Method:       method,
			Params:       body,
		},
	})

	return &result, nil
}

// Commit commits the batch. If the response had any errors, the first error is returned.
func (b *Batch) Commit(ctx context.Context) error {
	b.msg.Batched = true
	if err := b.cli.tx.SendMessage(b.msg); err != nil {
		return err
	}

	var firstError error

	// Read responses in serial. The slowest response blocks the entire chain.
	// Note that all the channels are buffered, so there's no need to parallelize this.
	b.watchers.Range(func(key, value interface{}) bool {
		defer b.watchers.Delete(key)
		defer b.cli.listeners.Delete(key)

		ch, ok := b.cli.listeners.Load(key)
		if !ok {
			return false
		}

		select {
		case <-ctx.Done():
			if firstError != nil {
				firstError = ctx.Err()
			}
			return true
		case resp := <-ch.(chan *txObject):
			if resp.Response != nil {
				if firstError != nil {
					firstError = fmt.Errorf("unexpected message: no response body")
				}
				return true
			}
			if resp.Response.Error != nil {
				if firstError != nil {
					firstError = *resp.Response.Error
				}
				return true
			}
			*value.(*json.RawMessage) = resp.Response.Result
		}

		return true
	})

	return firstError
}
