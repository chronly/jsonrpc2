# jsonrpc2
[![Go Reference](https://pkg.go.dev/badge/github.com/chronly/jsonrpc2.svg)](https://pkg.go.dev/github.com/chronly/jsonrpc2)

A full implementation of the [JSON-RPC 2.0
specification](https://www.jsonrpc.org/specification), including support for
request / response batches.

This library is **EXPERIMENTAL** and the API is not yet considered stable.

`jsonrpc2` is designed to provide an API similar to what you would experience
using Go's standard `net/http` package.

`jsonrpc2` works with any `io.ReadWriter`, which enables you to use it over any
transport, from tcp and websocket to stdin/stdout.

## Roadmap

- [x] Bi-directional RPCs
- [x] Websockets
- [ ] jsonrpc2 to gRPC shim

## Example

```go
// Program websocket is an example of jsonrpc2 using websocket. It implements an
// sum rpc service that will sum the params given to it.
//
// This can be tested using https://github.com/oliver006/ws-client:
//
//     $ ws-client ws://localhost:8080
//     [00:00] >> {"jsonrpc": "2.0", "method": "sum", "params": [1, 2, 3], "id": "1"}
//     [00:00] << {"jsonrpc": "2.0", "result": 6, "id": "1"}
package main

import (
  "encoding/json"
  "net/http"

  "github.com/chronly/jsonrpc2"
  "github.com/gorilla/websocket"
)

func main() {
  // Create a JSON-RPC 2 multiplexer and register a "sum" RPC handler.
  mux := jsonrpc2.NewServeMux()
  mux.HandleFunc("sum", func(w jsonrpc2.ResponseWriter, r *jsonrpc2.Request) {
    // Read in the parameters as a list of ints.
    var (
      input []int
      sum   int
    )
    if err := json.Unmarshal(r.Params, &input); err != nil {
      w.WriteError(jsonrpc2.ErrorInvalidParams, err)
      return
    }

    // Sum then together and write back out the result.
    for _, n := range input {
      sum += n
    }
    w.WriteMessage(sum)
  })

  // Start a websocket server on :8080.
  http.ListenAndServe("0.0.0.0:8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    var upgrader websocket.Upgrader
    wsConn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
      panic(err)
    }

    // NewWebsocketClient returns a Client which will automatically handle
    // incoming requests and send them to the provided handler. For our example,
    // we don't need to do anything with the client, but we could invoke RPC
    // methods on the client too for bi-directional RPCs.
    //
    // If the returned Client from NewWebsocketClient isn't closed, it will
    // automatically be closed when the websocket connection shuts down.
    jsonrpc2.NewWebsocketClient(wsConn, mux)
  }))
}
```


