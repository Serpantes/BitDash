package delivery

import (
	_ "embed"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"bitdash/backend/internal/cache"
)

const bitcraftMapOrigin = "https://bitcraftmap.com"

var bitcraftMapUpstreamHosts = map[string]string{
	"bcmap-api.bitjita.com": "https://bcmap-api.bitjita.com",
	"api.bitcraftmap.com":   "https://api.bitcraftmap.com",
	"exports.bitjita.com":   "https://exports.bitjita.com",
}

var (
	bitcraftMapCSPMeta       = regexp.MustCompile(`<meta\s+http-equiv="Content-Security-Policy"[^>]*>\s*`)
	bitcraftMapCloudflareJS  = regexp.MustCompile(`<script[^>]*>[^<]*cdn-cgi/challenge-platform[^<]*</script>\s*`)
	bitcraftMapCloudflareTag = regexp.MustCompile(`<script[^>]*cloudflareinsights\.com/beacon[^>]*></script>\s*`)
)

//go:embed bitcraftmap_bridge.js
var bitcraftMapBridgeJS string

const bitcraftMapFetchPatch = `<script id="bitdash-map-fetch-patch">
(function () {
  var upstreamHosts = ["bcmap-api.bitjita.com", "api.bitcraftmap.com", "exports.bitjita.com"];
  function rewriteUrl(url) {
    if (typeof url !== "string") return url;
    if (url.startsWith("/api/") || url.startsWith("/markers/")) {
      return "/api/map-proxy" + url;
    }
    try {
      var parsed = new URL(url, window.location.href);
      if (upstreamHosts.indexOf(parsed.hostname) !== -1) {
        return "/api/map-upstream/" + parsed.hostname + parsed.pathname + parsed.search;
      }
      if (parsed.origin === window.location.origin && (parsed.pathname.startsWith("/api/") || parsed.pathname.startsWith("/markers/"))) {
        return "/api/map-proxy" + parsed.pathname + parsed.search;
      }
    } catch (e) {}
    return url;
  }
  var origFetch = window.fetch;
  if (!origFetch) return;
  function fetchWithRetry(input, init, retries) {
    var nextUrl = typeof input === "string" ? rewriteUrl(input) : input;
    var request = typeof input === "string"
      ? nextUrl
      : (typeof nextUrl === "string" && nextUrl !== input.url ? new Request(nextUrl, input) : input);
    return origFetch.call(this, request, init).then(function (res) {
      if (retries > 0 && (res.status === 502 || res.status === 503 || res.status === 429)) {
        return new Promise(function (resolve) { setTimeout(resolve, 500); }).then(function () {
          return fetchWithRetry(input, init, retries - 1);
        });
      }
      return res;
    });
  }
  window.fetch = function (input, init) {
    return fetchWithRetry(input, init, 2);
  };
})();
</script>`

const bitcraftMapWebSocketPatch = `<script id="bitdash-map-websocket-patch">
(function () {
  var NativeWS = window.WebSocket;
  if (!NativeWS) return;
  function rewriteWsUrl(url) {
    if (typeof url !== "string") return url;
    if (url.indexOf("wss://live.bitjita.com") !== 0 && url.indexOf("ws://live.bitjita.com") !== 0) return url;
    var rest = url.replace(/^wss?:\/\/live\.bitjita\.com/, "") || "";
    if (rest.charAt(0) !== "/") rest = "/" + rest;
    var wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return wsProto + "//" + window.location.host + "/api/live-ws" + rest;
  }
  function PatchedWebSocket(url, protocols) {
    var next = rewriteWsUrl(url);
    if (protocols !== undefined) return new NativeWS(next, protocols);
    return new NativeWS(next);
  }
  PatchedWebSocket.prototype = NativeWS.prototype;
  PatchedWebSocket.CONNECTING = NativeWS.CONNECTING;
  PatchedWebSocket.OPEN = NativeWS.OPEN;
  PatchedWebSocket.CLOSING = NativeWS.CLOSING;
  PatchedWebSocket.CLOSED = NativeWS.CLOSED;
  window.WebSocket = PatchedWebSocket;
})();
</script>`

func NewBitcraftMapEmbedHandler() http.Handler {
	client := &http.Client{Timeout: 30 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path == "/map-embed" {
			target := "/map-embed/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		if r.URL.Path != "/map-embed/" {
			http.NotFound(w, r)
			return
		}

		upstreamQuery := r.URL.Query()
		for _, key := range []string{"bitdashMini", "bridge", "reload"} {
			upstreamQuery.Del(key)
		}
		target := bitcraftMapOrigin + "/"
		if encoded := upstreamQuery.Encode(); encoded != "" {
			target += "?" + encoded
		}

		body, status, err := fetchBitcraftMap(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		if status < 200 || status >= 300 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			w.Write(body)
			return
		}

		html := transformBitcraftMapEmbedHTML(string(body))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte(html))
	})
}

func transformBitcraftMapEmbedHTML(html string) string {
	html = bitcraftMapCSPMeta.ReplaceAllString(html, "")
	html = bitcraftMapCloudflareJS.ReplaceAllString(html, "")
	html = bitcraftMapCloudflareTag.ReplaceAllString(html, "")
	if idx := strings.Index(html, "</head>"); idx >= 0 {
		inject := `<script id="bitdash-map-bridge">` + bitcraftMapBridgeJS + `</script>` +
			bitcraftMapFetchPatch + bitcraftMapWebSocketPatch
		html = html[:idx] + inject + html[idx:]
	}
	return html
}

func NewBitcraftMapMirroredProxy(pathPrefix string) http.Handler {
	client := &http.Client{Timeout: 60 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, pathPrefix) {
			http.NotFound(w, r)
			return
		}

		target := bitcraftMapOrigin + r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		body, status, headers, err := fetchBitcraftMapWithHeaders(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func NewBitcraftMapAssetProxyHandler() http.Handler {
	client := &http.Client{Timeout: 60 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/map-embed")
		if path == "" {
			path = "/"
		}
		target := bitcraftMapOrigin + path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		body, status, headers, err := fetchBitcraftMapWithHeaders(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func NewBitcraftMapUpstreamProxyHandler(c *cache.Cache) http.Handler {
	client := &http.Client{Timeout: 120 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, "/api/map-upstream/")
		if rest == "" {
			http.NotFound(w, r)
			return
		}
		slash := strings.Index(rest, "/")
		if slash <= 0 {
			http.NotFound(w, r)
			return
		}
		host := rest[:slash]
		baseURL, ok := bitcraftMapUpstreamHosts[host]
		if !ok {
			http.Error(w, "upstream not allowed", http.StatusForbidden)
			return
		}

		target := baseURL + rest[slash:]
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		if r.Method == http.MethodGet && c != nil {
			if cached, ok := c.Get(target); ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(cached)
				return
			}
		}

		body, status, headers, err := fetchBitcraftMapWithHeaders(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		if r.Method == http.MethodGet && c != nil && status >= 200 && status < 300 {
			c.Set(target, body)
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func NewBitcraftMapProxyHandler() http.Handler {
	client := &http.Client{Timeout: 60 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/map-proxy")
		if path == "" {
			path = "/"
		}
		target := bitcraftMapOrigin + path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		body, status, headers, err := fetchBitcraftMapWithHeaders(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func fetchBitcraftMap(client *http.Client, method, target string) ([]byte, int, error) {
	body, status, _, err := fetchBitcraftMapWithHeaders(client, method, target)
	return body, status, err
}

func fetchBitcraftMapWithHeaders(client *http.Client, method, target string) ([]byte, int, http.Header, error) {
	req, err := http.NewRequest(method, target, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", "BitDash/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, nil, err
	}
	return body, resp.StatusCode, resp.Header, nil
}

func NewBitcraftMapFaviconHandler() http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		target := bitcraftMapOrigin + "/favicon.svg"
		body, status, headers, err := fetchBitcraftMapWithHeaders(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	}
}

func writeProxiedResponse(w http.ResponseWriter, method string, status int, headers http.Header, body []byte) {
	for key, values := range headers {
		for _, value := range values {
			if strings.EqualFold(key, "Content-Length") {
				continue
			}
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)
	if method == http.MethodHead {
		return
	}
	w.Write(body)
}
