package ports

import (
	"io"
	"net/http"
	"time"
)

//go:generate mockery --name HTTPClient --output ../../../test/mocks --outpkg mocks

// HTTPClient abstracts HTTP operations for testability
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	Head(url string) (*http.Response, error)
}

// HTTPClientConfig holds configuration for HTTP client
type HTTPClientConfig struct {
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	FollowRedirects bool
	UserAgent       string
}

// RealHTTPClient implements HTTPClient using standard HTTP client
type RealHTTPClient struct {
	client *http.Client
	config HTTPClientConfig
}

// NewRealHTTPClient creates a new real HTTP client implementation
func NewRealHTTPClient(config HTTPClientConfig) HTTPClient {
	return &RealHTTPClient{
		client: &http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if !config.FollowRedirects {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		config: config,
	}
}

// Do executes an HTTP request
func (c *RealHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Set User-Agent if configured
	if c.config.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	var resp *http.Response
	var err error

	// Retry logic
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err = c.client.Do(req)
		if err == nil {
			return resp, nil
		}

		// Don't retry on context cancellation
		if ctx := req.Context(); ctx != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}

		// Wait before retry (except on last attempt)
		if attempt < c.config.MaxRetries && c.config.RetryDelay > 0 {
			time.Sleep(c.config.RetryDelay)
		}
	}

	return resp, err
}

// Get performs an HTTP GET request
func (c *RealHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post performs an HTTP POST request
func (c *RealHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.Do(req)
}

// Head performs an HTTP HEAD request
func (c *RealHTTPClient) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}
