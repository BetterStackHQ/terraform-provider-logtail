package provider

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestErrorBody(t *testing.T) {
	body := []byte(`{"errors":{"secret_access_key":["is invalid"]}}`)

	t.Run("omits body by default", func(t *testing.T) {
		t.Setenv("TF_PROVIDER_LOGTAIL_LOG_INSECURE", "")
		if got := errorBody(body); got != "(set TF_PROVIDER_LOGTAIL_LOG_INSECURE=1 to include the response body, which may contain sensitive data)" {
			t.Errorf("errorBody() = %q, expected the body to be omitted", got)
		}
	})

	t.Run("includes body when insecure logging is enabled", func(t *testing.T) {
		t.Setenv("TF_PROVIDER_LOGTAIL_LOG_INSECURE", "1")
		if got := errorBody(body); got != string(body) {
			t.Errorf("errorBody() = %q, want %q", got, string(body))
		}
	})
}

func TestRateLimitRetryPolicy(t *testing.T) {
	reqURL := &url.URL{Scheme: "https", Host: "telemetry.betterstack.com", Path: "/api/v1/sources"}
	cases := []struct {
		name   string
		method string
		status int
		want   bool
	}{
		{name: "retries POST on 429", method: http.MethodPost, status: 429, want: true},
		{name: "does not retry POST on 500", method: http.MethodPost, status: 500, want: false},
		{name: "does not retry POST on 502", method: http.MethodPost, status: 502, want: false},
		{name: "retries GET on 500", method: http.MethodGet, status: 500, want: true},
		{name: "retries PATCH on 502", method: http.MethodPatch, status: 502, want: true},
		{name: "does not retry POST on 422", method: http.MethodPost, status: 422, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tc.status,
				Request:    &http.Request{Method: tc.method, URL: reqURL},
			}
			got, err := rateLimitRetryPolicy(context.Background(), resp, nil)
			if err != nil {
				t.Fatalf("rateLimitRetryPolicy returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("rateLimitRetryPolicy(%s %d) = %v, want %v", tc.method, tc.status, got, tc.want)
			}
		})
	}
}
