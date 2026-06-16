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

	out := transformBitcraftMapEmbedHTML(input, false)
	miniOut := transformBitcraftMapEmbedHTML(input, true)

	if strings.Contains(out, "Content-Security-Policy") {
		t.Fatal("expected CSP meta removed")
	}
	if !strings.Contains(out, "bitdash-hide-map-ui") {
		t.Fatal("expected hide UI style injected")
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
	if !strings.Contains(out, ".absolute.left-3.z-ui") {
		t.Fatal("expected coords/time/reset selector in CSS")
	}
	if strings.Contains(out, "bitdash-mini-map-ui") {
		t.Fatal("expected mini UI style only in mini mode")
	}
	if !strings.Contains(miniOut, "bitdash-mini-map-ui") {
		t.Fatal("expected mini UI style in mini mode")
	}
}
