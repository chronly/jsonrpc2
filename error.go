package jsonrpc2

import (
	"encoding/json"
	"fmt"
)

// Error messages used by RPC calls. Error messages from -32768 and -32000 are
// reserved by the JSON-RPC 2.0 framework.
const (
	ErrorParse          int = -32700
	ErrorInvalidRequest int = -32600
	ErrorMethodNotFound int = -32601
	ErrorInvalidParams  int = -32602
	ErrorInternal       int = -32603
)

var errorDesc = map[int]string{
	ErrorParse:          "Parse error",
	ErrorInvalidRequest: "Invalid Request",
	ErrorMethodNotFound: "Method not found",
	ErrorInvalidParams:  "Invalid params",
	ErrorInternal:       "Internal error",
}

// Error is a JSON-RPC 2.0 Error object. It may be returned by Invoke.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements error.
func (e Error) Error() string {
	var codeDesc string
	if d, ok := errorDesc[e.Code]; ok {
		codeDesc = d
	} else {
		codeDesc = fmt.Sprintf("RPC error (%d)", e.Code)
	}

	return codeDesc + ": " + e.Message
}
