package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// End-to-end (white-box): drive the real sourceCreate against a mock Sources API and assert the
// "connect your AWS account" hint is emitted on apply for an aws source, and not for other platforms.
func TestSourceCreateEmitsAWSHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		// Echo the request (the API returns the same immutable platform), plus computed fields.
		body = inject(t, body, "token", "tok")
		body = inject(t, body, "table_name", "x")
		body = inject(t, body, "team_id", 123456)
		body = inject(t, body, "ingesting_host", "in.logs.betterstack.com")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"1","attributes":%s}}`, body)))
	}))
	defer server.Close()

	cl, err := newClient(ClientConfig{BaseURL: server.URL, Token: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	res := newSourceResource()

	warned := func(raw map[string]interface{}) bool {
		d := schema.TestResourceDataRaw(t, res.Schema, raw)
		diags := sourceCreate(context.Background(), d, cl)
		for _, dg := range diags {
			if dg.Severity == diag.Error {
				t.Fatalf("unexpected error diag: %s: %s", dg.Summary, dg.Detail)
			}
			if dg.Severity == diag.Warning && strings.Contains(dg.Summary, "needs a connected AWS account") {
				return true
			}
		}
		return false
	}

	if !warned(map[string]interface{}{"name": "x", "platform": "aws"}) {
		t.Error("aws source should emit the connect-account hint")
	}
	if warned(map[string]interface{}{"name": "x", "platform": "docker"}) {
		t.Error("non-aws source should NOT emit the hint")
	}
	if warned(map[string]interface{}{"name": "x", "platform": "aws_cloudwatch"}) {
		t.Error("aws_cloudwatch is a different integration and should NOT emit the hint")
	}
}
