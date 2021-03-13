package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chronly/jsonrpc2"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWebsocket(t *testing.T) {
	var (
		upgrader websocket.Upgrader
		done     = make(chan struct{})
	)
	handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		require.NoError(t, err)

		cli := jsonrpc2.NewClient(FromConn(conn), jsonrpc2.HandlerFunc(func(w jsonrpc2.ResponseWriter, r *jsonrpc2.Request) {
			require.Equal(t, "test", r.Method)
			err := w.WriteMessage("Hello, world!")
			require.NoError(t, err)
		}))

		_ = cli.Wait(context.Background())
		close(done)
	})

	testSrv := httptest.NewServer(handler)
	t.Cleanup(testSrv.Close)

	// Retrieve the client and server websockets.
	clientWS, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s", testSrv.Listener.Addr().String()), nil)
	require.NoError(t, err)

	cli := jsonrpc2.NewClient(FromConn(clientWS), jsonrpc2.DefaultHandler)
	resp, err := cli.Invoke(context.Background(), "test", nil)
	require.NoError(t, err)
	require.Equal(t, `"Hello, world!"`, string(resp))

	clientWS.Close()
	<-done
}
