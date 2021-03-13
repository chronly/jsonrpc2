package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONRPC2(t *testing.T) {
	// Create a TCP server and a TCP client.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		require.NoError(t, err)
	}
	defer lis.Close()

	var router Router
	router.RegisterRoute("sum", HandlerFunc(func(w ResponseWriter, r *Request) {
		var nums []int
		err := json.Unmarshal(r.Params, &nums)
		if err != nil {
			w.WriteError(ErrorInvalidRequest, fmt.Errorf("invalid json: %w", err))
			return
		}

		var sum int
		for _, n := range nums {
			sum += n
		}

		w.WriteMessage(sum)
	}))

	srv := Server{Handler: &router}
	go srv.Serve(lis)

	cli, err := Dial(lis.Addr().String(), DefaultHandler)
	require.NoError(t, err)

	resp, err := cli.Invoke(context.Background(), "sum", []int{3, 5, 7})
	require.NoError(t, err)

	var res int
	require.NoError(t, json.Unmarshal(resp, &res))
	require.Equal(t, 3+5+7, res)
}
