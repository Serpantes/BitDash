package repository

import (
	"net/http"
	"time"
)

// ExternalClient calls the external API and returns the raw http.Response.
type ExternalClient struct {
	URL    string
	Client *http.Client
}

// NewExternalClient creates an ExternalClient with reasonable timeouts.
func NewExternalClient(url string) *ExternalClient {
	return &ExternalClient{URL: url, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Fetch performs a GET to the configured external URL while copying query params.
func (e *ExternalClient) Fetch(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, e.URL, nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = r.URL.RawQuery
	return e.Client.Do(req)
}
