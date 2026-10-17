package usecase

import "net/http"

type Fetcher interface {
	Fetch(r *http.Request) (*http.Response, error)
}
