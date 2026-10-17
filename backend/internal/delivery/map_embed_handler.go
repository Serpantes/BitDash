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

const (
	bitjitaSiteOrigin = "https://bitjita.com"
	mapEmbedPrefix    = "/map-embed"
	mapEmbedPage      = mapEmbedPrefix + "/map"
)

var mapUpstreamHosts = map[string]string{
	"mapdata.bitjita.com": "https://mapdata.bitjita.com",
}

var (
	mapCloudflareJS     = regexp.MustCompile(`<script[^>]*>[^<]*cdn-cgi/challenge-platform[^<]*</script>\s*`)
	mapCloudflareTag    = regexp.MustCompile(`<script[^>]*cloudflareinsights\.com/beacon[^>]*></script>\s*`)
	mapCloudflareAnchor = regexp.MustCompile(`<a href="[^"]*/cdn-cgi/content[^"]*"[^>]*>\s*</a>\s*`)
	mapDataURLEnv       = regexp.MustCompile(`"PUBLIC_MAP_DATA_URL":"https://mapdata\.bitjita\.com"`)
	mapPosthogEnv       = regexp.MustCompile(`"PUBLIC_POSTHOG_API_KEY":"[^"]*"`)

	// The map flies to a player as soon as one is watched (from a saved link or a search
	// pick) and fits the view to a plotted resource; the dashboard tile keeps its own view.
	mapAutoFlyWatch   = regexp.MustCompile(`this\.pendingFly=[A-Za-z_$][\w$]*(?:\[0\])?,`)
	mapAutoFitOverlay = regexp.MustCompile(`,fit:!0\}`)
)

func patchMapScript(body []byte) []byte {
	body = mapAutoFlyWatch.ReplaceAll(body, []byte(`this.pendingFly=null,`))
	return mapAutoFitOverlay.ReplaceAll(body, []byte(`,fit:!1}`))
}

//go:embed map_bridge.js
var mapBridgeJS string

// mapdata.bitjita.com only answers CORS for bitjita.com, so the page reads it through
// our upstream proxy; the env value is an expression so it resolves on this origin.
const mapDataURLProxied = `"PUBLIC_MAP_DATA_URL":location.origin+"/api/map-upstream/mapdata.bitjita.com"`

const mapFetchPatch = `<script id="bitdash-map-fetch-patch">
(function () {
  var upstreamHosts = ["mapdata.bitjita.com"];
  function rewriteUrl(url) {
    if (typeof url !== "string") return url;
    try {
      var parsed = new URL(url, window.location.href);
      if (upstreamHosts.indexOf(parsed.hostname) !== -1) {
        return "/api/map-upstream/" + parsed.hostname + parsed.pathname + parsed.search;
      }
      var site = parsed.hostname === "bitjita.com" || parsed.origin === window.location.origin;
      if (!site) return url;
      var path = parsed.pathname;
      if (path.indexOf("/api/") === 0) {
        if (path.indexOf("/api/bitjita/") === 0 || path.indexOf("/api/map-upstream/") === 0) return url;
        return "/api/bitjita" + path.slice(4) + parsed.search;
      }
      // the site serves icons and images from its root; mirror them under the embed prefix
      if (path.indexOf("/map-embed/") !== 0) return "/map-embed" + path + parsed.search;
    } catch (e) {}
    return url;
  }
  function rewriteSrc(value) {
    var text = String(value);
    return /^\/(?!\/)/.test(text) || text.indexOf("https://bitjita.com/") === 0 ? rewriteUrl(text) : value;
  }
  var srcDescriptor = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, "src");
  Object.defineProperty(HTMLImageElement.prototype, "src", {
    configurable: true,
    enumerable: srcDescriptor.enumerable,
    get: srcDescriptor.get,
    set: function (value) { srcDescriptor.set.call(this, rewriteSrc(value)); },
  });
  var setAttribute = Element.prototype.setAttribute;
  Element.prototype.setAttribute = function (name, value) {
    if (this instanceof HTMLImageElement && String(name).toLowerCase() === "src") value = rewriteSrc(value);
    return setAttribute.call(this, name, value);
  };
  var origFetch = window.fetch;
  if (!origFetch) return;
  window.fetch = function (input, init) {
    if (typeof input === "string" || input instanceof URL) return origFetch.call(this, rewriteUrl(String(input)), init);
    var next = rewriteUrl(input.url);
    return origFetch.call(this, next !== input.url ? new Request(next, input) : input, init);
  };
})();
</script>`

const mapWebSocketPatch = `<script id="bitdash-map-websocket-patch">
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
    var next = rewriteWsUrl(String(url));
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

// NewMapEmbedHandler serves bitjita.com/map from this origin, so the
// dashboard tile can talk to it (postMessage, hash state) and hide what it does not need.
func NewMapEmbedHandler() http.Handler {
	client := &http.Client{Timeout: 30 * time.Second}
	assets := newSiteProxy(bitjitaSiteOrigin, mapEmbedPrefix)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case mapEmbedPrefix, mapEmbedPrefix + "/":
			http.Redirect(w, r, mapEmbedPage, http.StatusFound)
			return
		case mapEmbedPage:
		default:
			assets.ServeHTTP(w, r)
			return
		}

		// embed=1 is the site's bare viewer without search, layers or live tracking; the full
		// page is used instead and the bridge hides the site header around it
		upstreamQuery := r.URL.Query()
		upstreamQuery.Del("reload")
		upstreamQuery.Del("embed")
		target := bitjitaSiteOrigin + "/map"
		if encoded := upstreamQuery.Encode(); encoded != "" {
			target += "?" + encoded
		}
		body, status, _, err := fetchUpstream(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if status < 200 || status >= 300 {
			w.WriteHeader(status)
			w.Write(body)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte(transformMapEmbedHTML(string(body))))
	})
}

func transformMapEmbedHTML(html string) string {
	html = mapCloudflareJS.ReplaceAllString(html, "")
	html = mapCloudflareTag.ReplaceAllString(html, "")
	html = mapCloudflareAnchor.ReplaceAllString(html, "")
	html = mapDataURLEnv.ReplaceAllString(html, mapDataURLProxied)
	html = mapPosthogEnv.ReplaceAllString(html, `"PUBLIC_POSTHOG_API_KEY":""`)
	if idx := strings.Index(html, "</head>"); idx >= 0 {
		inject := `<script id="bitdash-map-bridge">` + mapBridgeJS + `</script>` + mapFetchPatch + mapWebSocketPatch
		html = html[:idx] + inject + html[idx:]
	}
	return html
}

// newSiteProxy mirrors origin's files (the app's /_app chunks, icons, manifest) under prefix.
func newSiteProxy(origin, prefix string) http.Handler {
	client := &http.Client{Timeout: 60 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, prefix)
		if path == "" || path == r.URL.Path {
			http.NotFound(w, r)
			return
		}
		target := origin + path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		body, status, headers, err := fetchUpstream(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		if status == http.StatusOK && strings.HasSuffix(path, ".js") {
			body = patchMapScript(body)
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func NewMapUpstreamProxyHandler(c *cache.Cache) http.Handler {
	client := &http.Client{Timeout: 120 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, "/api/map-upstream/")
		slash := strings.Index(rest, "/")
		if rest == "" || slash <= 0 {
			http.NotFound(w, r)
			return
		}
		host := rest[:slash]
		baseURL, ok := mapUpstreamHosts[host]
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

		body, status, headers, err := fetchUpstream(client, r.Method, target)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		// only JSON is kept in memory: binary tiles are content-hashed and cached by the browser
		if r.Method == http.MethodGet && c != nil && status >= 200 && status < 300 &&
			strings.HasPrefix(headers.Get("Content-Type"), "application/json") {
			c.Set(target, body)
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	})
}

func fetchUpstream(client *http.Client, method, target string) ([]byte, int, http.Header, error) {
	req, err := http.NewRequest(method, target, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; BitDash/1.0)")

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

func NewFaviconHandler() http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, status, headers, err := fetchUpstream(client, r.Method, bitjitaSiteOrigin+"/favicon-32x32.png")
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		writeProxiedResponse(w, r.Method, status, headers, body)
	}
}

func writeProxiedResponse(w http.ResponseWriter, method string, status int, headers http.Header, body []byte) {
	for key, values := range headers {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Set-Cookie") ||
			strings.HasPrefix(strings.ToLower(key), "access-control-") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)
	if method == http.MethodHead {
		return
	}
	w.Write(body)
}
