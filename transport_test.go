package jsonrpc2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTransport_Marshal tests the JSON conversion for the various transport types.
func TestTransport_Marshal(t *testing.T) {
	tt := []struct {
		name   string
		input  interface{}
		expect string
	}{
		{
			name: "request rpc",
			input: &txRequest{
				Notification: false,
				ID:           newUndefinedID(),
				Method:       "hello",
				Params:       json.RawMessage(`[0,1,2]`),
			},
			expect: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": [0,1,2],
				"id": null
			}`,
		},
		{
			name: "request with id",
			input: &txRequest{
				Notification: false,
				ID:           newStringID("12345"),
				Method:       "hello",
				Params:       json.RawMessage(`{}`),
			},
			expect: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": {},
				"id": "12345"
			}`,
		},
		{
			name: "request notification",
			input: &txRequest{
				Notification: true,
				Method:       "hello",
				Params:       json.RawMessage(`{}`),
			},
			expect: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": {}
			}`,
		},

		{
			name: "success response",
			input: &txResponse{
				ID:     newNullID(),
				Result: json.RawMessage(`{}`),
			},
			expect: `{
				"jsonrpc": "2.0",
				"id": null,
				"result": {}
			}`,
		},
		{
			name: "error response",
			input: &txResponse{
				ID: newStringID("12345"),
				Error: &Error{
					Code:    ErrorInternal,
					Message: "some error",
				},
			},
			expect: `{
				"jsonrpc": "2.0",
				"id": "12345",
				"error": {
					"code": -32603,
					"message": "some error"
				}
			}`,
		},

		{
			name: "request object",
			input: &txObject{
				Request: &txRequest{
					Notification: true,
					Method:       "test",
					Params:       json.RawMessage(`[]`),
				},
			},
			expect: `{
				"jsonrpc": "2.0",
				"method": "test",
				"params": []
			}`,
		},
		{
			name: "response object undefined ID",
			input: &txObject{
				Response: &txResponse{
					Result: json.RawMessage(`[]`),
				},
			},
			expect: `{
				"jsonrpc": "2.0",
				"result": []
			}`,
		},
		{
			name: "response object null ID",
			input: &txObject{
				Response: &txResponse{
					Result: json.RawMessage(`[]`),
					ID:     newNullID(),
				},
			},
			expect: `{
				"jsonrpc": "2.0",
				"result": [],
				"id": null
			}`,
		},

		{
			name: "non batched message",
			input: &txMessage{
				Objects: []*txObject{{
					Request: &txRequest{
						ID:     newStringID("1"),
						Method: "hello",
						Params: json.RawMessage(`[]`),
					},
				}},
			},
			expect: `{
				"jsonrpc": "2.0",
				"id": "1",
				"method": "hello",
				"params": []
			}`,
		},
		{
			name: "batched message",
			input: &txMessage{
				Batched: true,
				Objects: []*txObject{{
					Request: &txRequest{
						ID:     newStringID("1"),
						Method: "hello",
						Params: json.RawMessage(`[]`),
					},
				}},
			},
			expect: `[{
				"jsonrpc": "2.0",
				"id": "1",
				"method": "hello",
				"params": []
			}]`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			output, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.JSONEq(t, tc.expect, string(output))
		})
	}
}

func TestTransport_Unmarshal(t *testing.T) {
	tt := []struct {
		name      string
		input     string
		unmarshal func(bb []byte) (interface{}, error)
		expect    interface{}
	}{
		{
			name: "request rpc",
			input: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": [0,1,2],
				"id": null
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txRequest
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txRequest{
				Notification: false,
				ID:           newNullID(),
				Method:       "hello",
				Params:       json.RawMessage(`[0,1,2]`),
			},
		},
		{
			name: "request with id",
			input: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": {},
				"id": "12345"
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txRequest
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txRequest{
				Notification: false,
				ID:           newStringID("12345"),
				Method:       "hello",
				Params:       json.RawMessage(`{}`),
			},
		},
		{
			name: "request notification",
			input: `{
				"jsonrpc": "2.0",
				"method": "hello",
				"params": {}
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txRequest
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txRequest{
				Notification: true,
				Method:       "hello",
				Params:       json.RawMessage(`{}`),
			},
		},

		{
			name: "success response",
			input: `{
				"jsonrpc": "2.0",
				"id": null,
				"result": {}
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txResponse
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txResponse{
				ID:     newNullID(),
				Result: json.RawMessage(`{}`),
			},
		},
		{
			name: "error response",
			input: `{
				"jsonrpc": "2.0",
				"id": "12345",
				"error": {
					"code": -32603,
					"message": "some error"
				}
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txResponse
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txResponse{
				ID: newStringID("12345"),
				Error: &Error{
					Code:    ErrorInternal,
					Message: "some error",
				},
			},
		},

		{
			name: "request object",
			input: `{
				"jsonrpc": "2.0",
				"method": "test",
				"params": []
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txObject
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txObject{
				Request: &txRequest{
					Notification: true,
					Method:       "test",
					Params:       json.RawMessage(`[]`),
				},
			},
		},
		{
			name: "response object undefined ID",
			input: `{
				"jsonrpc": "2.0",
				"result": []
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txObject
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txObject{
				Response: &txResponse{
					Result: json.RawMessage(`[]`),
				},
			},
		},
		{
			name: "response object null ID",
			input: `{
				"jsonrpc": "2.0",
				"result": [],
				"id": null
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txObject
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txObject{
				Response: &txResponse{
					Result: json.RawMessage(`[]`),
					ID:     newNullID(),
				},
			},
		},

		{
			name: "non batched message",
			input: `{
				"jsonrpc": "2.0",
				"id": "1",
				"method": "hello",
				"params": []
			}`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txMessage
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txMessage{
				Objects: []*txObject{{
					Request: &txRequest{
						ID:     newStringID("1"),
						Method: "hello",
						Params: json.RawMessage(`[]`),
					},
				}},
			},
		},
		{
			name: "batched message",
			input: `[{
				"jsonrpc": "2.0",
				"id": "1",
				"method": "hello",
				"params": []
			}]`,
			unmarshal: func(bb []byte) (interface{}, error) {
				var msg txMessage
				err := json.Unmarshal(bb, &msg)
				return &msg, err
			},
			expect: &txMessage{
				Batched: true,
				Objects: []*txObject{{
					Request: &txRequest{
						ID:     newStringID("1"),
						Method: "hello",
						Params: json.RawMessage(`[]`),
					},
				}},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.unmarshal([]byte(tc.input))
			require.NoError(t, err)
			require.Equal(t, tc.expect, output)
		})
	}
}
