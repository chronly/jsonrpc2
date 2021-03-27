package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type transportError struct {
	Err error
}

func (te *transportError) Unwrap() error {
	return te.Err
}

func (te *transportError) Error() string {
	return te.Err.Error()
}

// transport is a transport for JSON-RPC 2.0 message.
type transport struct {
	rw io.ReadWriter
}

// newTransport can read and write JSON-RPC 2.0 messages over a ReadWriter.
func newTransport(rw io.ReadWriter) *transport {
	return &transport{rw: rw}
}

// ReadMessage reads the next txMessage from the transport.
func (t *transport) ReadMessage() (txMessage, error) {
	var msg txMessage
	err := json.NewDecoder(t.rw).Decode(&msg)
	if err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			err = &transportError{Err: err}
		}

		var ue *json.UnmarshalTypeError
		if errors.As(err, &ue) {
			err = &transportError{Err: err}
		}
	}
	return msg, err
}

// SendMessage sends a message over the transport.
func (t *transport) SendMessage(msg txMessage) error {
	return json.NewEncoder(t.rw).Encode(&msg)
}

func (t *transport) SendError(id id, err *Error) error {
	return t.SendMessage(txMessage{
		Objects: []*txObject{{
			Response: &txResponse{ID: id, Error: err},
		}},
	})
}

// Close closes the transport. If the rw given to newTransport implements
// io.Closer, it will be closed.
func (t *transport) Close() error {
	if c, ok := t.rw.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// txMessage is a transport message, which can be batched.
type txMessage struct {
	Batched bool
	Objects []*txObject
}

func (m *txMessage) UnmarshalJSON(bb []byte) error {
	// Most messages won't be batched, so try the non-batched form first.
	var obj txObject
	if err := json.Unmarshal(bb, &obj); err == nil {
		m.Batched = false
		m.Objects = []*txObject{&obj}
		return nil
	}

	// Fallback to trying a batch.
	var objs []*txObject
	err := json.Unmarshal(bb, &objs)
	if err == nil {
		m.Batched = true
		m.Objects = objs
		return nil
	}

	return err
}

func (m *txMessage) MarshalJSON() ([]byte, error) {
	if m.Batched {
		return json.Marshal(m.Objects)
	}

	if len(m.Objects) != 1 {
		return nil, fmt.Errorf("must be one object for a non-batched message")
	}
	return json.Marshal(m.Objects[0])
}

// txObjects are either requests or responses.
type txObject struct {
	Request  *txRequest
	Response *txResponse
}

func (m *txObject) UnmarshalJSON(bb []byte) error {
	var (
		req  txRequest
		resp txResponse
	)

	reqErr := json.Unmarshal(bb, &req)
	if reqErr == nil {
		m.Request = &req
		return nil
	}

	respErr := json.Unmarshal(bb, &resp)
	if respErr == nil {
		m.Response = &resp
		return nil
	}

	return fmt.Errorf("invalid json-rpc 2.0 message: %s for request and %s for response", reqErr, respErr)
}

func (o *txObject) MarshalJSON() ([]byte, error) {
	if o.Request != nil && o.Response != nil {
		return nil, fmt.Errorf("invalid object: only request or response may be set")
	}
	if o.Request == nil && o.Response == nil {
		return nil, fmt.Errorf("invalid object: either request or response must be set")
	}

	if o.Request != nil {
		return json.Marshal(o.Request)
	}
	return json.Marshal(o.Response)
}

// txRequest is a Request object as specified by JSON-RPC 2.0.
type txRequest struct {
	// If Notification is true, then ID must be nil.
	Notification bool
	ID           id
	Method       string
	Params       json.RawMessage
}

func (r *txRequest) UnmarshalJSON(bb []byte) error {
	type plain struct {
		Version string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      id              `json:"id"`
	}
	var p plain

	dec := json.NewDecoder(bytes.NewReader(bb))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}

	if p.Version != "2.0" {
		return fmt.Errorf("invalid jsonrpc version: %s", p.Version)
	}

	r.Notification = p.ID.IsUndefined()
	r.Method = p.Method
	r.Params = p.Params
	r.ID = p.ID
	return nil
}

func (r *txRequest) MarshalJSON() ([]byte, error) {
	if r.Notification {
		type notification struct {
			Version string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var n notification
		n.Version = "2.0"
		n.Method = r.Method
		n.Params = r.Params
		return json.Marshal(n)
	} else {
		type plain struct {
			Version string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
			ID      id              `json:"id"`
		}
		var p plain
		p.Version = "2.0"
		p.Method = r.Method
		p.Params = r.Params
		p.ID = r.ID
		return json.Marshal(p)
	}
}

type txResponse struct {
	// ID must be nil the request couldn't be parsed.
	ID     id
	Result json.RawMessage
	Error  *Error
}

func (r *txResponse) UnmarshalJSON(bb []byte) error {
	type plain struct {
		Version string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *Error          `json:"error"`
		ID      id              `json:"id"`
	}
	var p plain

	dec := json.NewDecoder(bytes.NewReader(bb))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}

	if p.Version != "2.0" {
		return fmt.Errorf("invalid jsonrpc version: %s", p.Version)
	}

	if len(p.Result) > 0 && p.Error != nil {
		return fmt.Errorf("only one of result and error may be set")
	} else if p.Result == nil && p.Error == nil {
		return fmt.Errorf("one of result or error must be set")
	}

	r.ID = p.ID
	r.Result = p.Result
	r.Error = p.Error
	return nil
}

func (r *txResponse) MarshalJSON() ([]byte, error) {
	if len(r.Result) > 0 && r.Error != nil {
		return nil, fmt.Errorf("only one of result and error may be set")
	} else if r.Result == nil && r.Error == nil {
		return nil, fmt.Errorf("one of result or error must be set")
	}

	if r.Error == nil {
		type plain struct {
			Version string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			ID      *id             `json:"id,omitempty"`
		}
		var p plain
		p.Version = "2.0"
		p.Result = r.Result
		p.ID = &r.ID
		if r.ID.IsUndefined() {
			p.ID = nil
		}
		return json.Marshal(p)
	} else {
		type plain struct {
			Version string `json:"jsonrpc"`
			Error   *Error `json:"error"`
			ID      *id    `json:"id,omitempty"`
		}
		var p plain
		p.Version = "2.0"
		p.Error = r.Error
		p.ID = &r.ID
		if r.ID.IsUndefined() {
			p.ID = nil
		}
		return json.Marshal(p)
	}
}
