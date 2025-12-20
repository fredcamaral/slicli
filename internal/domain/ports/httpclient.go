package ports

import (
	"io"
	"net/http"
)

//go:generate mockery --name HTTPClient --output ../../../test/mocks --outpkg mocks

// HTTPClient abstracts HTTP operations for testability
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	Head(url string) (*http.Response, error)
}
