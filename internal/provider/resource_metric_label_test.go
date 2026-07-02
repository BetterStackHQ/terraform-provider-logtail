package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestResourceMetricAsLabel covers issue #79: aggregations is optional, so a
// metric expression with no aggregations is a Label rather than a Metric. It
// also exercises converting a Label into a Metric and back, asserting that the
// request body always carries the aggregations field (empty for a Label) so the
// backend can switch kinds in place instead of leaving the old value untouched.
func TestResourceMetricAsLabel(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	var lastPostBody atomic.Value
	var lastPatchBody atomic.Value
	id.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			lastPostBody.Store(body)
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, id.Load(), data.Load())))
		case r.Method == http.MethodPatch && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastPatchBody.Store(body)
			data.Store(body)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	// aggregationsInBody returns a check that decodes the most recent request body
	// (POST or PATCH) and asserts the aggregations field equals want. A non-nil
	// empty slice (a Label) is required - a missing/null field would let the
	// backend keep the previous aggregations.
	aggregationsInBody := func(store *atomic.Value, want []string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			raw, _ := store.Load().([]byte)
			var got metric
			if err := json.Unmarshal(raw, &got); err != nil {
				return fmt.Errorf("body not valid JSON: %v (body=%s)", err, string(raw))
			}
			if got.Aggregations == nil {
				return fmt.Errorf("body missing `aggregations` field (body=%s)", string(raw))
			}
			if fmt.Sprint(*got.Aggregations) != fmt.Sprint(want) {
				return fmt.Errorf("body aggregations = %v, want %v (body=%s)", *got.Aggregations, want, string(raw))
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create a Label by omitting aggregations entirely.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "service_name"
					sql_expression = "getJSON(raw, 'service_name')"
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "0"),
					// The create request must still carry an (empty) aggregations list.
					aggregationsInBody(&lastPostBody, []string{}),
				),
			},
			// Step 2 - no changes, plan must be empty (no perpetual diff on the
			// optional/empty list).
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "service_name"
					sql_expression = "getJSON(raw, 'service_name')"
				}`,
				PlanOnly: true,
			},
			// Step 3 - convert the Label into a Metric by adding aggregations.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "service_name"
					sql_expression = "getJSON(raw, 'service_name')"
					aggregations   = ["uniq"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.0", "uniq"),
					aggregationsInBody(&lastPatchBody, []string{"uniq"}),
				),
			},
			// Step 4 - convert it back into a Label by clearing aggregations. The
			// PATCH body must send an explicit empty list, not omit the field.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "service_name"
					sql_expression = "getJSON(raw, 'service_name')"
					aggregations   = []
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "0"),
					aggregationsInBody(&lastPatchBody, []string{}),
				),
			},
		},
	})
}
