package delivery

import (
	"strings"
	"testing"
)

func TestTransformBitcraftMapEmbedHTML(t *testing.T) {
	input := `<!doctype html>
<html>
<head>
<meta http-equiv="Content-Security-Policy" content="default-src 'self';" />
</head>
<body>
<script>
__sveltekit_rjoz6 = {
  base: new URL(".", location).pathname.slice(0, -1),
};
import("./_app/immutable/entry/start.js");
import("./_app/immutable/entry/app.js");
</script>
<script>(function(){function c(){var a=document.createElement('script');a.src='/cdn-cgi/challenge-platform/scripts/jsd/main.js';}})();</script>
<script defer src="https://static.cloudflareinsights.com/beacon.min.js/v833ccba57c9e4d2798f2e76cebdd09a11778172276447" crossorigin="anonymous"></script>
</body>
</html>`

	out := transformBitcraftMapEmbedHTML(input)

	if strings.Contains(out, "Content-Security-Policy") {
		t.Fatal("expected CSP meta removed")
	}
	if !strings.Contains(out, `<script id="bitdash-map-bridge">`) {
		t.Fatal("expected map bridge injected")
	}
	if strings.Index(out, "bitdash-map-bridge") > strings.Index(out, "</head>") {
		t.Fatal("expected map bridge in <head>, before the app's modules")
	}
	if !strings.Contains(out, "addInitHook") || !strings.Contains(out, "postMessage") {
		t.Fatal("expected bridge to hook Leaflet and talk over postMessage")
	}
	if !strings.Contains(out, "bitdash-map-fetch-patch") {
		t.Fatal("expected fetch patch injected")
	}
	if !strings.Contains(out, "bcmap-api.bitjita.com") {
		t.Fatal("expected bcmap-api host in fetch patch")
	}
	if !strings.Contains(out, "/api/map-upstream/") {
		t.Fatal("expected map-upstream rewrite in fetch patch")
	}
	if !strings.Contains(out, "bitdash-map-websocket-patch") {
		t.Fatal("expected websocket patch injected")
	}
	if !strings.Contains(out, "/api/live-ws") {
		t.Fatal("expected live-ws rewrite in websocket patch")
	}
	if !strings.Contains(out, "new URL(\".\", location).pathname.slice(0, -1)") {
		t.Fatal("expected original sveltekit base computation preserved")
	}
	if !strings.Contains(out, "import(\"./_app/immutable/entry/start.js\")") {
		t.Fatal("expected relative module imports preserved")
	}
	if strings.Contains(out, "cdn-cgi/challenge-platform") {
		t.Fatal("expected cloudflare challenge script removed")
	}
	if strings.Contains(out, "cloudflareinsights.com/beacon") {
		t.Fatal("expected cloudflare beacon removed")
	}
	if !strings.Contains(out, ".relative.h-viewport > *:not(.z-map)") {
		t.Fatal("expected the site's own UI hidden by the bridge")
	}
}
