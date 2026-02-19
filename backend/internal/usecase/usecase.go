package usecase

import "net/http"

// Fetcher defines the use-case boundary for retrieving external data.
type Fetcher interface {
	Fetch(r *http.Request) (*http.Response, error)
}
