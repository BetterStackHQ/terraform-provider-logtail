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
// "connect your Azure tenant" hint is emitted on apply for an azure source (with a direct link to
// the source's Ingest tab, built from the API-returned team id), and not for other platforms.
func TestSourceCreateEmitsAzureHint(t *testing.T) {
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

	hintDetail := func(raw map[string]interface{}) string {
		d := schema.TestResourceDataRaw(t, res.Schema, raw)
		diags := sourceCreate(context.Background(), d, cl)
		for _, dg := range diags {
			if dg.Severity == diag.Error {
				t.Fatalf("unexpected error diag: %s: %s", dg.Summary, dg.Detail)
			}
			if dg.Severity == diag.Warning && strings.Contains(dg.Summary, "needs a connected Azure tenant") {
				return dg.Detail
			}
		}
		return ""
	}

	detail := hintDetail(map[string]interface{}{"name": "x", "platform": "azure"})
	if detail == "" {
		t.Error("azure source should emit the connect-tenant hint")
	}
	if !strings.Contains(detail, "https://telemetry.betterstack.com/team/123456/sources/1/data-ingestion") {
		t.Errorf("hint should link the source's Ingest tab, got: %s", detail)
	}
	if hintDetail(map[string]interface{}{"name": "x", "platform": "docker"}) != "" {
		t.Error("non-azure source should NOT emit the hint")
	}
	if hintDetail(map[string]interface{}{"name": "x", "platform": "azure_logs"}) != "" {
		t.Error("azure_logs is a different integration and should NOT emit the hint")
	}
}
