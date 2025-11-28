package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/context/ctxhttp"
)

type client struct {
	baseURL          string
	errorsBaseURL    string
	warehouseBaseURL string
	token            string
	httpClient       *http.Client
	userAgent        string
}

type option func(c *client)

func withHTTPClient(httpClient *http.Client) option {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func withUserAgent(userAgent string) option {
	return func(c *client) {
		c.userAgent = userAgent
	}
}

func newClient(baseURL, token string, opts ...option) (*client, error) {
	c := client{
		baseURL:          baseURL,
		errorsBaseURL:    "https://errors.betterstack.com",
		warehouseBaseURL: "https://warehouse.betterstack.com",
		token:            token,
		httpClient:       http.DefaultClient,
	}
	// Override with test URL if baseURL is not the production URL
	if baseURL != "https://logs.betterstack.com" {
		c.errorsBaseURL = baseURL
		c.warehouseBaseURL = baseURL
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c, nil
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
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", baseURL, path), body)
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
	return ctxhttp.Do(ctx, c.httpClient, req)
}
