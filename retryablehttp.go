package retryablehttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/condrove10/retryablehttp/backoffpolicy"
	"github.com/go-playground/validator/v10"
)

// Client is a struct that contains the context, http client, attempts, delay, strategy, and policy
type Client struct {
	context context.Context
	// Could be an issue if someone messes with the http.Client
	httpClient *http.Client
	attempts   uint32
	delay      time.Duration
	strategy   backoffpolicy.Strategy
	policy     func(resp *http.Response, err error) error
}

// New creates a new instance of the Client struct
func New(ctx context.Context, client *http.Client, attempts uint32, delay time.Duration, strategy backoffpolicy.Strategy, policy func(resp *http.Response, err error) error) *Client {
	return &Client{
		context:    ctx,
		httpClient: client,
		attempts:   attempts,
		delay:      delay,
		strategy:   strategy,
		policy:     policy,
	}
}

// Post sends a POST request to the specified URL
func (c *Client) Post(url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(url, http.MethodPost, body, headers)
}

// Get sends a GET request to the specified URL
func (c *Client) Get(url string, headers map[string]string) (*http.Response, error) {
	return c.Do(url, http.MethodGet, make([]byte, 0), headers)
}

// Do sends an HTTP request to the specified URL with the specified method
func (c *Client) Do(url, method string, body []byte, headers map[string]string) (*http.Response, error) {
	if err := validator.New().Var(url, "required,http_url"); err != nil {
		return nil, fmt.Errorf("url validation failed: %w", err)
	}

	header := http.Header{}
	for k, v := range headers {
		header.Add(k, v)
	}

	req, err := http.NewRequestWithContext(c.context, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header = header
	var resp = &http.Response{}

	backoffpolicy.BackoffPolicy(c.strategy, c.attempts, c.delay, func(attempt uint32) error {
		if c.context.Err() != nil {
			err := fmt.Errorf("retryable http call context closed: %w", c.context.Err())
			return err
		}

		if attempt > 0 {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, err = c.httpClient.Do(req)

		return c.policy(resp, err)
	})

	return resp, err
}
