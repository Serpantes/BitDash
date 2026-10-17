package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

func TestWebSocketProxyRelaysBothWays(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := bitcraftMapWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			kind, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(kind, append([]byte(r.URL.Path+":"), msg...)) //nolint:errcheck
		}
	}))
	defer upstream.Close()
	proxy := httptest.NewServer(newWebSocketProxyHandler(wsURL(upstream, ""), "/api/live-ws"))
	defer proxy.Close()

	client, _, err := websocket.DefaultDialer.Dial(wsURL(proxy, "/api/live-ws"), nil)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer client.Close()
	client.WriteMessage(websocket.TextMessage, []byte("hi")) //nolint:errcheck
	client.SetReadDeadline(time.Now().Add(5 * time.Second))  //nolint:errcheck
	_, msg, err := client.ReadMessage()
	if err != nil || string(msg) != "/:hi" {
		t.Fatalf("got %q, %v", msg, err)
	}
}

func TestWebSocketProxyForwardsUpstreamClose(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := bitcraftMapWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "restart"), time.Now().Add(time.Second)) //nolint:errcheck
		time.Sleep(100 * time.Millisecond)
		conn.Close()
	}))
	defer upstream.Close()
	proxy := httptest.NewServer(newWebSocketProxyHandler(wsURL(upstream, ""), "/api/live-ws"))
	defer proxy.Close()

	client, _, err := websocket.DefaultDialer.Dial(wsURL(proxy, "/api/live-ws"), nil)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer client.Close()
	client.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, _, err = client.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseTryAgainLater) {
		t.Fatalf("expected forwarded close 1013, got %v", err)
	}
}

func TestWebSocketProxyRejectsWhenUpstreamDown(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	proxy := httptest.NewServer(newWebSocketProxyHandler(wsURL(upstream, ""), "/api/live-ws"))
	defer proxy.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURL(proxy, "/api/live-ws"), nil)
	if err == nil {
		t.Fatal("expected handshake failure")
	}
	if resp == nil || resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %v", resp)
	}
}
