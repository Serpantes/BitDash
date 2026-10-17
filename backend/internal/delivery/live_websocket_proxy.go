package delivery

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const liveBitjitaWSOrigin = "wss://live.bitjita.com"

const (
	liveWSPingEvery = 25 * time.Second
	liveWSPongWait  = 70 * time.Second
	liveWSWriteWait = 10 * time.Second
)

var bitcraftMapWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var liveBitjitaWSDialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 15 * time.Second,
}

func NewLiveBitjitaWebSocketProxyHandler(pathPrefix string) http.HandlerFunc {
	return newWebSocketProxyHandler(liveBitjitaWSOrigin, pathPrefix)
}

// Both legs are pinged so a silently dead peer (sleeping laptop, dropped NAT entry,
// stuck upstream) closes the pair instead of leaving the browser on an open socket
// that never delivers another event.
func newWebSocketProxyHandler(origin, pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			http.Error(w, "expected websocket", http.StatusBadRequest)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, pathPrefix)
		if path == "" {
			path = "/"
		}
		upstream := origin + path
		if r.URL.RawQuery != "" {
			upstream += "?" + r.URL.RawQuery
		}

		// Dial first: a failed upstream then surfaces as a failed handshake, which the
		// browser reports as an error and retries, rather than as an accepted socket.
		upstreamConn, _, err := liveBitjitaWSDialer.DialContext(r.Context(), upstream, nil)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer upstreamConn.Close()

		clientConn, err := bitcraftMapWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer clientConn.Close()

		relayWebSockets(clientConn, upstreamConn)
	}
}

func relayWebSockets(a, b *websocket.Conn) {
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }

	for _, conn := range []*websocket.Conn{a, b} {
		keepAlive(conn)
		go pingUntil(conn, done, stop)
	}
	go func() { pumpWebSocket(a, b); stop() }()
	go func() { pumpWebSocket(b, a); stop() }()
	<-done
}

func keepAlive(conn *websocket.Conn) {
	conn.SetReadDeadline(time.Now().Add(liveWSPongWait)) //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(liveWSPongWait))
	})
}

func pingUntil(conn *websocket.Conn, done <-chan struct{}, stop func()) {
	ticker := time.NewTicker(liveWSPingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(liveWSWriteWait)); err != nil {
				stop()
				return
			}
		}
	}
}

func pumpWebSocket(from, to *websocket.Conn) {
	for {
		messageType, payload, err := from.ReadMessage()
		if err != nil {
			forwardClose(to, err)
			return
		}
		from.SetReadDeadline(time.Now().Add(liveWSPongWait)) //nolint:errcheck
		to.SetWriteDeadline(time.Now().Add(liveWSWriteWait)) //nolint:errcheck
		if err := to.WriteMessage(messageType, payload); err != nil {
			return
		}
	}
}

// forwardClose passes the peer's close code on, so the other side sees a clean close
// it can react to; codes that may not be sent on the wire become "going away".
func forwardClose(to *websocket.Conn, err error) {
	code, text := websocket.CloseGoingAway, "peer gone"
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		switch closeErr.Code {
		case websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure, websocket.CloseTLSHandshake:
		default:
			code, text = closeErr.Code, closeErr.Text
		}
	}
	to.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(time.Second)) //nolint:errcheck
}
