package jsonrpc2

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestNewWebsocketClient(t *testing.T) {
	var (
		upgrader websocket.Upgrader
	)
	handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		require.NoError(t, err)

		NewWebsocketClient(conn, HandlerFunc(func(w ResponseWriter, r *Request) {
			require.Equal(t, "test", r.Method)
			err := w.WriteMessage("Hello, world!")
			require.NoError(t, err)
		}))
	})

	testSrv := httptest.NewServer(handler)
	t.Cleanup(testSrv.Close)

	// Retrieve the client and server websockets.
	clientWS, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s", testSrv.Listener.Addr().String()), nil)
	require.NoError(t, err)

	cli := NewWebsocketClient(clientWS, DefaultHandler)
	resp, err := cli.Invoke(context.Background(), "test", nil)
	require.NoError(t, err)
	require.Equal(t, `"Hello, world!"`, string(resp))

	clientWS.Close()
}
