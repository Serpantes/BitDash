package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"bitdash/backend/internal/repository"
)

const (
	visitorCookie = "bitdash_visitor"
	visitorMaxAge = 400 * 24 * 60 * 60
)

type visitorKey struct{}

// WithVisitor gives every browser an anonymous random ID in a long-lived
// cookie, so per-visitor data needs no accounts.
func WithVisitor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id string
		if c, err := r.Cookie(visitorCookie); err == nil && repository.ValidVisitorID(c.Value) {
			id = c.Value
		} else {
			raw := make([]byte, 16)
			if _, err := rand.Read(raw); err != nil {
				http.Error(w, "visitor id: "+err.Error(), http.StatusInternalServerError)
				return
			}
			id = hex.EncodeToString(raw)
			http.SetCookie(w, &http.Cookie{
				Name:     visitorCookie,
				Value:    id,
				Path:     "/",
				MaxAge:   visitorMaxAge,
				HttpOnly: true,
				Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
				// Lax keeps other sites from sending a PUT with this cookie.
				SameSite: http.SameSiteLaxMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), visitorKey{}, id)))
	})
}

// VisitorID returns the ID WithVisitor put on the request, or "".
func VisitorID(r *http.Request) string {
	id, _ := r.Context().Value(visitorKey{}).(string)
	return id
}
