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

func TestAWSSourceNeedsAccountHint(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		inline   bool
		want     bool
	}{
		{"aws, no inline creds -> hint", "aws", false, true},
		{"aws, inline creds set -> no hint", "aws", true, false},
		{"non-aws platform -> no hint", "docker", false, false},
		{"aws_cloudwatch is a different integration -> no hint", "aws_cloudwatch", false, false},
		{"http -> no hint", "http", false, false},
	}
	for _, c := range cases {
		if got := awsSourceNeedsAccountHint(c.platform, c.inline); got != c.want {
			t.Errorf("%s: awsSourceNeedsAccountHint(%q, %v) = %v, want %v", c.name, c.platform, c.inline, got, c.want)
		}
	}
}

// End-to-end (white-box): drive the real sourceCreate against a mock Sources API and assert the
// "connect your AWS account" hint is emitted on apply for an aws source with no linkage, and
// suppressed when creds are provided inline or the platform isn't aws.
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
			if dg.Severity == diag.Warning && strings.Contains(dg.Summary, "without a connected AWS account") {
				return true
			}
		}
		return false
	}

	if !warned(map[string]interface{}{"name": "x", "platform": "aws"}) {
		t.Error("aws source without linkage should emit the connect-account hint")
	}
	if warned(map[string]interface{}{"name": "x", "platform": "aws", "aws_role_arn": "arn:aws:iam::1:role/x", "aws_external_id": "ext"}) {
		t.Error("aws source connecting inline should NOT emit the hint")
	}
	if warned(map[string]interface{}{"name": "x", "platform": "aws", "aws_account_id": "42"}) {
		t.Error("aws source reusing an account id should NOT emit the hint")
	}
	if warned(map[string]interface{}{"name": "x", "platform": "docker"}) {
		t.Error("non-aws source should NOT emit the hint")
	}
}
