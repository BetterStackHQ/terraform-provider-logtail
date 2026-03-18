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

func TestResourceDashboardChart(t *testing.T) {
	var dashboardData atomic.Value
	var chartData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		dashboardID := "1"
		chartID := "10"

		switch {
		// Dashboard CRUD
		case r.Method == http.MethodPost && r.RequestURI == "/api/v2/dashboards":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}
			reqData["created_at"] = "2023-01-01T00:00:00Z"
			reqData["updated_at"] = "2023-01-01T00:00:00Z"
			if _, ok := reqData["date_range_from"]; !ok {
				reqData["date_range_from"] = "now-3h"
			}
			if _, ok := reqData["date_range_to"]; !ok {
				reqData["date_range_to"] = "now"
			}
			if _, ok := reqData["refresh_interval"]; !ok {
				reqData["refresh_interval"] = 0
			}
			respData, _ := json.Marshal(reqData)
			dashboardData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, dashboardID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/"+dashboardID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, dashboardID, dashboardData.Load().([]byte))))

		case r.Method == http.MethodDelete && r.RequestURI == "/api/v2/dashboards/"+dashboardID:
			w.WriteHeader(http.StatusNoContent)

		// Chart CRUD
		case r.Method == http.MethodPost && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/charts":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}
			reqData["created_at"] = "2023-01-01T00:00:00Z"
			reqData["updated_at"] = "2023-01-01T00:00:00Z"
			// Add position defaults
			if _, ok := reqData["x"]; !ok {
				reqData["x"] = 0
			}
			if _, ok := reqData["y"]; !ok {
				reqData["y"] = 0
			}
			if _, ok := reqData["w"]; !ok {
				reqData["w"] = 6
			}
			if _, ok := reqData["h"]; !ok {
				reqData["h"] = 4
			}
			if _, ok := reqData["description"]; !ok {
				reqData["description"] = ""
			}
			if _, ok := reqData["settings"]; !ok {
				reqData["settings"] = map[string]interface{}{}
			}
			// Add query IDs and defaults
			if queries, ok := reqData["queries"].([]interface{}); ok {
				for i, q := range queries {
					if qMap, ok := q.(map[string]interface{}); ok {
						qMap["id"] = i + 1
						if _, ok := qMap["source_variable"]; !ok {
							qMap["source_variable"] = "source"
						}
						if _, ok := qMap["name"]; !ok {
							qMap["name"] = ""
						}
						if _, ok := qMap["where_condition"]; !ok {
							qMap["where_condition"] = ""
						}
						if _, ok := qMap["static_text"]; !ok {
							qMap["static_text"] = ""
						}
					}
				}
			}
			respData, _ := json.Marshal(reqData)
			chartData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, chartID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/charts/"+chartID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, chartID, chartData.Load().([]byte))))

		case r.Method == http.MethodPatch && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/charts/"+chartID:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(chartData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patch["updated_at"] = "2023-01-02T00:00:00Z"
			patched, _ := json.Marshal(patch)
			chartData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, chartID, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/charts/"+chartID:
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create chart
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Test Dashboard"
				}

				resource "logtail_dashboard_chart" "this" {
					dashboard_id = logtail_dashboard.this.id
					chart_type   = "line_chart"
					name         = "Request Rate"
					x = 0
					y = 0
					w = 6
					h = 4

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT count(*) AS value FROM logs"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_chart.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "chart_type", "line_chart"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "name", "Request Rate"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "x", "0"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "y", "0"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "w", "6"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "h", "4"),
					resource.TestCheckResourceAttrSet("logtail_dashboard_chart.this", "created_at"),
				),
			},
			// Step 2 - update chart
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Test Dashboard"
				}

				resource "logtail_dashboard_chart" "this" {
					dashboard_id = logtail_dashboard.this.id
					chart_type   = "line_chart"
					name         = "Request Rate Updated"
					x = 0
					y = 0
					w = 12
					h = 6

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT count(*) AS value FROM logs WHERE level = 'error'"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_chart.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "name", "Request Rate Updated"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "w", "12"),
					resource.TestCheckResourceAttr("logtail_dashboard_chart.this", "h", "6"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_dashboard_chart.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return "1/10", nil
				},
			},
		},
	})
}
