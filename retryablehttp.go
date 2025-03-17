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

// ClientOption represents a functional option for configuring the retryable HTTP client.
type ClientOption func(*Client)

// Client represents an HTTP client that automatically retries requests on failures.
type Client struct {
	context    context.Context
	httpClient *http.Client
	attempts   uint32
	delay      time.Duration
	strategy   backoffpolicy.Strategy
	policy     func(resp *http.Response, err error) error
}

// Default HTTP client with 5 retry attemps and 1 second timeout.
var defaultClient = Client{
	context:    context.Background(),
	httpClient: http.DefaultClient,
	attempts:   5,
	delay:      1 * time.Second,
	strategy:   backoffpolicy.StrategyLinear,
	policy: func(resp *http.Response, err error) error {
		if err != nil {
			return err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil
	},
}

// New creates and returns a new Client instance configured with the provided options.
// The default client configuration is used if none is specified.
func New(ctx context.Context, opts ...ClientOption) *Client {
	client := defaultClient
	client.context = ctx

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithAttempts(attempts uint32) ClientOption {
	return func(c *Client) {
		c.attempts = attempts
	}
}

func WithDelay(delay time.Duration) ClientOption {
	return func(c *Client) {
		c.delay = delay
	}
}

func WithStrategy(strategy backoffpolicy.Strategy) ClientOption {
	return func(c *Client) {
		c.strategy = strategy
	}
}

func WithPolicy(policy func(resp *http.Response, err error) error) ClientOption {
	return func(c *Client) {
		c.policy = policy
	}
}

// Post sends a POST request to the specified URL with the provided body and headers.
// It uses the underlying retry mechanism to ensure that transient errors are retried
// according to the configured policy.
func (c *Client) Post(url string, body []byte, headers map[string]string) (*http.Response, error) {
	return c.Do(url, http.MethodPost, body, headers)
}

// Get sends a GET request to the specified URL with the provided headers.
// As GET requests do not typically have a body, an empty payload is used.
func (c *Client) Get(url string, headers map[string]string) (*http.Response, error) {
	return c.Do(url, http.MethodGet, nil, headers)
}

// Do performs an HTTP request with the specified method, URL, body, and headers.
// It validates the URL, constructs the HTTP request with context support, and
// manages retry attempts using the configured backoff strategy and policy.
//
// The function returns the HTTP response if successful, or an error if all
// retry attempts fail.
func (c *Client) Do(url, method string, body []byte, headers map[string]string) (*http.Response, error) {
	// Validate URL format using go-playground/validator.
	if err := validator.New().Var(url, "required,http_url"); err != nil {
		return nil, fmt.Errorf("url validation failed: %w", err)
	}

	// Prepare HTTP headers from the provided map.
	header := http.Header{}
	for k, v := range headers {
		header.Add(k, v)
	}

	// Create a new HTTP request with context to support cancellation and timeouts.
	req, err := http.NewRequestWithContext(c.context, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header = header

	var resp = &http.Response{}

	// Execute the HTTP request with retry logic using the configured backoff policy.
	err = backoffpolicy.BackoffPolicy(c.strategy, c.attempts, c.delay, func(attempt uint32) error {
		// Ensure that the context is still active before each retry attempt.
		if c.context.Err() != nil {
			err := fmt.Errorf("retryable http call context closed: %w", c.context.Err())
			return err
		}

		// For retries beyond the first attempt, reset the request body.
		if attempt > 0 {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		// Perform the HTTP request.
		resp, err = c.httpClient.Do(req)

		// Use the custom policy to determine if a retry should occur.
		return c.policy(resp, err)
	})

	if err != nil {
		return nil, fmt.Errorf("backoff policy expired: %w", err)
	}

	return resp, err
}
