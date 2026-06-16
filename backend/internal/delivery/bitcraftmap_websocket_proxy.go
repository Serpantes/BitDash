package delivery

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

const liveBitjitaWSOrigin = "wss://live.bitjita.com"

var bitcraftMapWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewLiveBitjitaWebSocketProxyHandler proxies browser WebSocket connections to live.bitjita.com.
func NewLiveBitjitaWebSocketProxyHandler(pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			http.Error(w, "expected websocket", http.StatusBadRequest)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, pathPrefix)
		if path == "" {
			path = "/"
		}
		upstream := liveBitjitaWSOrigin + path
		if r.URL.RawQuery != "" {
			upstream += "?" + r.URL.RawQuery
		}

		clientConn, err := bitcraftMapWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer clientConn.Close()

		upstreamConn, _, err := websocket.DefaultDialer.Dial(upstream, nil)
		if err != nil {
			return
		}
		defer upstreamConn.Close()

		errc := make(chan error, 2)
		pump := func(from, to *websocket.Conn) {
			for {
				messageType, payload, err := from.ReadMessage()
				if err != nil {
					errc <- err
					return
				}
				if err := to.WriteMessage(messageType, payload); err != nil {
					errc <- err
					return
				}
			}
		}

		go pump(clientConn, upstreamConn)
		go pump(upstreamConn, clientConn)
		<-errc
	}
}
