package delivery

import (
	"strings"
	"testing"
)

func TestTransformMapEmbedHTML(t *testing.T) {
	input := `<!doctype html>
<html lang="en" class="dark">
<head>
<link href="./_app/immutable/assets/0.css" rel="stylesheet">
</head>
<body data-sveltekit-preload-data="tap">
<a href="https://bitjita.com/cdn-cgi/content?id=abc" aria-hidden="true" rel="nofollow noopener" style="display: none !important"></a>
<div style="display: contents">
<script>
__sveltekit_x = {
  base: new URL(".", location).pathname.slice(0, -1),
  env: {"PUBLIC_MAP_DATA_URL":"https://mapdata.bitjita.com","PUBLIC_POSTHOG_API_KEY":"phc_secret"}
};
import("./_app/immutable/entry/start.js");
</script>
</div>
<script>(function(){function c(){var a=document.createElement('script');a.src='/cdn-cgi/challenge-platform/scripts/jsd/main.js';}})();</script>
</body>
</html>`

	out := transformMapEmbedHTML(input)

	for _, want := range []string{
		`<script id="bitdash-map-bridge">`,
		"bitdash-map-fetch-patch",
		"bitdash-map-websocket-patch",
		"/api/live-ws",
		`"PUBLIC_MAP_DATA_URL":location.origin+"/api/map-upstream/mapdata.bitjita.com"`,
		`"PUBLIC_POSTHOG_API_KEY":""`,
		`new URL(".", location).pathname.slice(0, -1)`,
		`import("./_app/immutable/entry/start.js")`,
		"--app-header-h: 0px",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in transformed page", want)
		}
	}
	for _, gone := range []string{"cdn-cgi/challenge-platform", "cdn-cgi/content", "phc_secret"} {
		if strings.Contains(out, gone) {
			t.Fatalf("expected %q removed", gone)
		}
	}
	if strings.Index(out, "bitdash-map-bridge") > strings.Index(out, "</head>") {
		t.Fatal("expected the bridge in <head>, before the app's modules")
	}
}

func TestPatchMapScriptDropsAutoFly(t *testing.T) {
	in := `e.follow===!0&&(this.followId=t[0]),this.pendingFly=t[0],this.lookupLastKnown(t)` +
		`n.follow&&(this.followId=e),this.pendingFly=e,this.map.setCategoryVisible()` +
		`b.addOverlayType({kind:"res",id:e.id},{regions:Ee(),fit:!0}),Oe()` +
		`b.select({kind:"marker",marker:e.marker},{fly:!0})`
	out := string(patchMapScript([]byte(in)))
	if strings.Contains(out, "pendingFly=t[0]") || strings.Contains(out, "pendingFly=e,") {
		t.Fatalf("expected watched players not to be flown to: %s", out)
	}
	if strings.Contains(out, "fit:!0") {
		t.Fatalf("expected plotted resources not to refit the view: %s", out)
	}
	if !strings.Contains(out, "{fly:!0}") {
		t.Fatal("expected picking a place in search to still go there")
	}
}
