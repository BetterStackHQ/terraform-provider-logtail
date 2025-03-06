package provider

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceMetric(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	id.Store(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/sources/source123/metrics"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, id.Load(), data.Load())))
		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var sourceID = "source123"
	var name = "Test Metric"
	var sqlExpression = "JSONExtract(json, 'duration_ms', 'Nullable(Float)')"
	var metricType = "float64_delta"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_metric" "this" {
					source_id      = "%s"
					name           = "%s"
					sql_expression = "%s"
					type           = "%s"
					aggregations   = ["avg", "p50"]
				}
				`, sourceID, name, sqlExpression, metricType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "source_id", sourceID),
					resource.TestCheckResourceAttr("logtail_metric.this", "name", name),
					resource.TestCheckResourceAttr("logtail_metric.this", "sql_expression", sqlExpression),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", metricType),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "2"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.0", "avg"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.1", "p50"),
				),
				PreConfig: func() {
					t.Log("step 1")
				},
			},
			// Step 2 - test validation on type and aggregations
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_metric" "this" {
					source_id      = "%s"
					name           = "%s"
					sql_expression = "%s"
					type           = "mystery_column"
					aggregations   = ["min", "max", "best"]
				}
				`, sourceID, name, sqlExpression),
				Check:       resource.ComposeTestCheckFunc(),
				ExpectError: regexp.MustCompile(`expected type to be one of \["string_low_cardinality" "int64_delta" "float64_delta" "datetime64_delta" "boolean"], got mystery_column(.|\n)*expected aggregations\.2 to be one of \["avg" "count" "uniq" "max" "min" "anyLast" "sum" "p50" "p90" "p95" "p99"], got best`),
				PreConfig: func() {
					t.Log("step 2")
				},
			},
			// Step 3 - update, should change ID because it's a recreation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_metric" "this" {
					source_id      = "%s"
					name           = "%s"
					sql_expression = "%s"
					type           = "%s"
					aggregations   = ["min", "max"]
				}
				`, sourceID, name, sqlExpression, metricType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "2"),
					resource.TestCheckResourceAttr("logtail_metric.this", "source_id", sourceID),
					resource.TestCheckResourceAttr("logtail_metric.this", "name", name),
					resource.TestCheckResourceAttr("logtail_metric.this", "sql_expression", sqlExpression),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", metricType),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "2"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.0", "min"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.1", "max"),
				),
				PreConfig: func() {
					t.Log("step 3")
				},
			},
			// Step 4 - make no changes, check plan is empty
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_metric" "this" {
					source_id      = "%s"
					name           = "%s"
					sql_expression = "%s"
					type           = "%s"
					aggregations   = ["min", "max"]
				}
				`, sourceID, name, sqlExpression, metricType),
				PlanOnly: true,
				PreConfig: func() {
					t.Log("step 4")
				},
			},
		},
	})
}
