package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceMetricImport(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/sources/source123/metrics"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"42","attributes":%s}}`, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"42","attributes":%s}],"pagination":{"next":null}}`, data.Load())))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/42":
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	config := `
	provider "logtail" {
		api_token = "foo"
	}

	resource "logtail_metric" "this" {
		source_id      = "source123"
		name           = "Test Metric"
		sql_expression = "JSONExtract(json, 'duration_ms', 'Nullable(Float)')"
		aggregations   = ["avg"]
	}
	`

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create.
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "42"),
					resource.TestCheckResourceAttr("logtail_metric.this", "source_id", "source123"),
				),
			},
			// Step 2 - import using the composite source_id/metric_id ID.
			{
				ResourceName:      "logtail_metric.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "source123/42",
			},
			// Step 3 - importing a bare metric ID must fail with a helpful message.
			{
				ResourceName:  "logtail_metric.this",
				ImportState:   true,
				ImportStateId: "42",
				ExpectError:   regexp.MustCompile(`invalid metric ID format "42", expected 'source_id/metric_id'`),
			},
		},
	})
}
