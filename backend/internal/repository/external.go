package repository

import (
	"net/http"
	"time"
)

type ExternalClient struct {
	URL    string
	Client *http.Client
}

func NewExternalClient(url string) *ExternalClient {
	return &ExternalClient{URL: url, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (e *ExternalClient) Fetch(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, e.URL, nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = r.URL.RawQuery
	return e.Client.Do(req)
}
