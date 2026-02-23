package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/time/rate"
)

func rateLimitRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if err != nil {
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}

	if resp.StatusCode == 429 {
		return true, nil
	}

	return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
}

type client struct {
	baseURL          string
	errorsBaseURL    string
	warehouseBaseURL string
	token            string
	retryClient      *retryablehttp.Client
	userAgent        string
	rateLimiter      *rate.Limiter
}

type ClientConfig struct {
	BaseURL      string
	Token        string
	UserAgent    string
	HTTPClient   *http.Client
	RetryMax     int
	RetryWaitMin time.Duration
	RetryWaitMax time.Duration
	RateLimit    int // requests per second, 0 = no limit
	RateBurst    int // burst size for rate limiter, 0 = use default
}

func newClient(config ClientConfig) (*client, error) {
	// Set reasonable bounds for max retries
	if config.RetryMax < 0 || config.RetryMax > 10 {
		config.RetryMax = 10
	}
	// Set default wait times
	if config.RetryWaitMin == 0 {
		config.RetryWaitMin = 1 * time.Second
	}
	if config.RetryWaitMax == 0 {
		config.RetryWaitMax = 30 * time.Second
	}

	// Create retry client
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = config.RetryMax
	retryClient.RetryWaitMin = config.RetryWaitMin
	retryClient.RetryWaitMax = config.RetryWaitMax
	retryClient.CheckRetry = rateLimitRetryPolicy
	retryClient.Backoff = retryablehttp.DefaultBackoff

	// Use custom HTTP client if provided
	if config.HTTPClient != nil {
		retryClient.HTTPClient = config.HTTPClient
	}

	// Use hclog for Terraform-compatible logging (visible with TF_LOG=DEBUG)
	logLevel := hclog.LevelFromString(os.Getenv("TF_LOG"))
	if logLevel == hclog.NoLevel {
		logLevel = hclog.Off
	}
	retryClient.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "logtail-api",
		Level: logLevel,
	})

	// Create rate limiter if specified
	var rateLimiter *rate.Limiter
	if config.RateLimit > 0 {
		burst := config.RateBurst
		if burst <= 0 {
			// Default burst: allow accumulating up to 2 seconds worth of requests
			// This handles Terraform's pattern of idle-then-busy well
			burst = config.RateLimit * 2
			if burst < 10 {
				burst = 10 // Minimum burst of 10 for reasonable performance
			}
		}
		rateLimiter = rate.NewLimiter(rate.Limit(config.RateLimit), burst)
	}

	errorsBaseURL := "https://errors.betterstack.com"
	warehouseBaseURL := "https://warehouse.betterstack.com"
	// Override with test URL if baseURL is not the production URL
	if config.BaseURL != "https://telemetry.betterstack.com" {
		errorsBaseURL = config.BaseURL
		warehouseBaseURL = config.BaseURL
	}

	return &client{
		baseURL:          config.BaseURL,
		errorsBaseURL:    errorsBaseURL,
		warehouseBaseURL: warehouseBaseURL,
		token:            config.Token,
		retryClient:      retryClient,
		userAgent:        config.UserAgent,
		rateLimiter:      rateLimiter,
	}, nil
}

func (c *client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL, path, nil)
}

func (c *client) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL, path, body)
}

func (c *client) Patch(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPatch, c.baseURL, path, body)
}

func (c *client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, c.baseURL, path, nil)
}

func (c *client) GetWithBaseURL(ctx context.Context, baseURL, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, baseURL, path, nil)
}

func (c *client) PostWithBaseURL(ctx context.Context, baseURL, path string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, baseURL, path, body)
}

func (c *client) PatchWithBaseURL(ctx context.Context, baseURL, path string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, http.MethodPatch, baseURL, path, body)
}

func (c *client) DeleteWithBaseURL(ctx context.Context, baseURL, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, baseURL, path, nil)
}

func (c *client) TelemetryBaseURL() string {
	return c.baseURL
}

func (c *client) ErrorsBaseURL() string {
	return c.errorsBaseURL
}

func (c *client) WarehouseBaseURL() string {
	return c.warehouseBaseURL
}

func (c *client) do(ctx context.Context, method, baseURL, path string, body io.Reader) (*http.Response, error) {
	// Apply rate limiting if configured
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
	}

	req, err := retryablehttp.NewRequest(method, fmt.Sprintf("%s%s", baseURL, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if method == http.MethodPost || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.retryClient.Do(req.WithContext(ctx))
}
