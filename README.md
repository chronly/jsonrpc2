# jsonrpc2

A full implementation of the [JSON-RPC 2.0
specification](https://www.jsonrpc.org/specification), including support for
request / response batches.

This library is **pre 1.0** and the API is not considered stable.

`jsonrpc2` is designed to provide an API similar to what you would experience
using Go's standard `net/http` package.

## Roadmap

- [ ] Bi-directional RPCs
- [ ] Websockets
- [ ] jsonrpc2 to gRPC shim

## Example

```go
package main

import (
  "context"
  "encoding/json"
  "fmt"
  "log"
  "net"

  "github.com/chronly/jsonrpc2"
)

func main() {
  // Start a TCP server to accept jsonrpc2 messages.
  lis, err := net.Listen("tcp", "0.0.0.0:0")
  if err != nil {
    log.Fatalln(err)
  }
  defer lis.Close()

  var (
    srv    jsonrpc2.Server
    router jsonrpc2.Router
  )

  // Register a function to be called any time the "Sum" rpc method is invoked.
  router.RegisterRoute("Sum", jsonrpc2.HandlerFunc(func(w jsonrpc2.ResponseWriter, r *jsonrpc2.Request) {
    var input []int
    err := json.Unmarshal(r.Params, &input)
    if err != nil {
      w.WriteError(jsonrpc2.ErrorInvalidRequest, fmt.Errorf("invalid json: %w", err))
      return
    }

    var sum int
    for _, n := range input {
      sum += n
    }
    w.WriteMessage(sum)
  }))

  // Set the server's handler to our router so our callback gets invoked.
  srv.Handler = &router

  // Connect to our server as a client.
  cli, err := jsonrpc2.Dial(lis.Addr().String())
  if err != nil {
    log.Fatalln(err)
  }

  // Invoke the Sum RPC method and ask it to calculate the sum of 3, 5, and 7.
  resp, err := cli.Invoke(context.Background(), "Sum", []int{3, 5, 7})
  if err != nil {
    log.Fatalln(err)
  }

  var res int
  if err := json.Unmarshal(resp, &res); err != nil {
    log.Fatalln(err)
  }

  // Prints "Got resuslt: 15"
  fmt.Printf("Got result: %d\n", res)
}
```


