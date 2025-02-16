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

// ClientOptions defines the configuration options for creating a new Client instance.
// It includes the number of retry attempts, the initial delay between retries,
// and the strategy and policy used for exponential or linear backoff.
type ClientOptions struct {
	Attempts        uint32
	Delay           time.Duration
	BackoffStrategy backoffpolicy.Strategy
	BackoffPolicy   func(resp *http.Response, err error) error
}

// Client represents an HTTP client that automatically retries requests on failures.
// It encapsulates a standard HTTP client along with context management and retry logic.
type Client struct {
	context    context.Context
	httpClient *http.Client
	attempts   uint32
	delay      time.Duration
	strategy   backoffpolicy.Strategy
	policy     func(resp *http.Response, err error) error
}

// New creates and returns a new Client instance configured with the provided context,
// HTTP client, and client options. This function initializes the retryable HTTP client,
// enabling retry logic based on the specified parameters.
func New(ctx context.Context, client *http.Client, opts *ClientOptions) *Client {
	return &Client{
		context:    ctx,
		httpClient: client,
		attempts:   opts.Attempts,
		delay:      opts.Delay,
		strategy:   opts.BackoffStrategy,
		policy:     opts.BackoffPolicy,
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
