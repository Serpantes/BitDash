package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bitdash/backend/internal/repository"
)

func TestDashboardHandlerPerVisitor(t *testing.T) {
	stores, err := repository.NewDashboardStores(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	h := WithVisitor(NewDashboardHandler(stores))

	// First visit: the response hands out a cookie.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != visitorCookie || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v", cookies)
	}
	alice := cookies[0]

	put := httptest.NewRequest(http.MethodPut, "/api/dashboard", strings.NewReader(`{"baseRev":0,"blocks":[{"inventoryId":"a"}]}`))
	put.AddCookie(alice)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, put)
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}

	get := func(c *http.Cookie) string {
		req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
		if c != nil {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	if body := get(alice); !strings.Contains(body, `"inventoryId":"a"`) {
		t.Fatalf("alice sees %s", body)
	}
	if body := get(nil); strings.Contains(body, `"inventoryId":"a"`) {
		t.Fatalf("a new visitor sees alice's blocks: %s", body)
	}
	if body := get(&http.Cookie{Name: visitorCookie, Value: "../../etc"}); strings.Contains(body, `"inventoryId":"a"`) {
		t.Fatalf("a forged cookie sees alice's blocks: %s", body)
	}
}
