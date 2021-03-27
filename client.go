package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"go.uber.org/atomic"
)

// ClientOpt is an option function that can be passed to Dial and NewClient.
type ClientOpt func(*Client)

// WithClientLogger sets the Client to use a logger.
func WithClientLogger(l log.Logger) ClientOpt {
	return func(c *Client) {
		if l == nil {
			l = log.NewNopLogger()
		} else {
			c.log = l
		}
	}
}

type Client struct {
	log log.Logger

	tx *transport

	// listeners holds channels waiting for a response to a specific
	// message ID. It is implemented a a map of int64 to a chan of
	// *txObject.
	//
	// The channels stored in listeners are NEVER closed, but cleaned up
	// by the Go GC once the goroutine that populated listeners removes
	// the entry.
	listeners sync.Map

	nextID  *atomic.Int64
	handler Handler

	done chan struct{}
}

// Dial creates a connection to the target server using TCP. Handler will
// be invoked for each request received from the other side.
func Dial(target string, handler Handler, opts ...ClientOpt) (*Client, error) {
	var d net.Dialer
	nc, err := d.Dial("tcp", target)
	if err != nil {
		return nil, fmt.Errorf("failed dialing to server: %w", err)
	}
	return NewClient(nc, handler, opts...), nil
}

// NewClient creates a client and starts reading messages from the provided
// io.ReadWriter. The given handler will be invoked for each request
// and notification that is read over rw.
//
// If rw implements io.Closer, it will be closed when the Client is closed.
func NewClient(rw io.ReadWriter, handler Handler, opts ...ClientOpt) *Client {
	if handler == nil {
		handler = DefaultHandler
	}

	cli := &Client{
		log: log.NewNopLogger(),

		tx:      newTransport(rw),
		handler: handler,
		nextID:  atomic.NewInt64(0),

		done: make(chan struct{}),
	}
	for _, o := range opts {
		o(cli)
	}
	go cli.processMessages()
	return cli
}

// Close closes the underlying transport.
func (c *Client) Close() error {
	return c.tx.Close()
}

// Done returns a channel that indicates when the client has closed.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// processMessages runs in the background and handles incoming messages from
// the server.
func (c *Client) processMessages() {
	defer close(c.done)

	for {
		batch, err := c.tx.ReadMessage()
		if err != nil {
			var txErr *transportError
			if errors.As(err, &txErr) {
				_ = c.tx.SendError(nil, &Error{
					Code:    ErrorInvalidRequest,
					Message: err.Error(),
				})
				continue
			}

			level.Info(c.log).Log("msg", "closing client", "err", err)
			_ = c.Close()
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
					level.Warn(c.log).Log("msg", "received error message", "msg", msg)
					continue Objects
				}

				msgID := convertID(msg.Response.ID)
				lis, ok := c.listeners.Load(msgID)
				if !ok {
					// The listener either never existed or went away.
					level.Warn(c.log).Log("msg", "missing listener for message response", "id", msgID)
					continue Objects
				}

				select {
				case lis.(chan *txObject) <- msg:
					// Listener got message, continue as normal
				case <-time.After(500 * time.Millisecond):
					level.Warn(c.log).Log("msg", "unresponsive listener", "id", msgID)
					break
				}
			}
		}

		if len(resp.Objects) > 0 {
			if err := c.tx.SendMessage(resp); err != nil {
				level.Warn(c.log).Log("msg", "error sending message, closing client", "err", err)
				return
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
		Notification: req.Notification,

		Method: req.Method,
		Params: req.Params,
		Client: c,
	})

	if ww.resp.Result == nil {
		ww.resp.Result = []byte{}
	}
	return ww.resp
}

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
