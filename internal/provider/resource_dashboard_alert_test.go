package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceDashboardAlert(t *testing.T) {
	var dashboardData atomic.Value
	var chartData atomic.Value
	var alertData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		dashboardID := "1"
		chartID := "10"
		alertID := "100"
		alertPrefix := fmt.Sprintf("/api/v2/dashboards/%s/charts/%s/alerts", dashboardID, chartID)

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

		case r.Method == http.MethodDelete && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/charts/"+chartID:
			w.WriteHeader(http.StatusNoContent)

		// Alert CRUD
		case r.Method == http.MethodPost && r.RequestURI == alertPrefix:
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
			reqData["series_names"] = []string{}
			reqData["source_platforms"] = []string{}
			if _, ok := reqData["source_mode"]; !ok {
				reqData["source_mode"] = "source_variable"
			}
			if _, ok := reqData["paused"]; !ok {
				reqData["paused"] = false
			}
			reqData["paused_reason"] = nil
			if _, ok := reqData["incident_per_series"]; !ok {
				reqData["incident_per_series"] = false
			}
			if _, ok := reqData["check_period"]; !ok {
				reqData["check_period"] = 300
			}
			if _, ok := reqData["call"]; !ok {
				reqData["call"] = false
			}
			if _, ok := reqData["sms"]; !ok {
				reqData["sms"] = false
			}
			if _, ok := reqData["email"]; !ok {
				reqData["email"] = false
			}
			if _, ok := reqData["push"]; !ok {
				reqData["push"] = false
			}
			if _, ok := reqData["critical_alert"]; !ok {
				reqData["critical_alert"] = false
			}
			respData, _ := json.Marshal(reqData)
			alertData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == alertPrefix+"/"+alertID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, alertData.Load().([]byte))))

		case r.Method == http.MethodPatch && strings.HasPrefix(r.RequestURI, alertPrefix+"/"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(alertData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patch["updated_at"] = "2023-01-02T00:00:00Z"
			patched, _ := json.Marshal(patch)
			alertData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, patched)))

		case r.Method == http.MethodDelete && strings.HasPrefix(r.RequestURI, alertPrefix+"/"):
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
			// Step 1 - create alert
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

				resource "logtail_dashboard_alert" "this" {
					dashboard_id        = logtail_dashboard.this.id
					chart_id            = logtail_dashboard_chart.this.id
					name                = "High Request Rate"
					alert_type          = "threshold"
					operator            = "higher_than"
					value               = 100
					check_period        = 60
					query_period        = 300
					confirmation_period = 60

					email = true
					push  = true
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_alert.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "name", "High Request Rate"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "alert_type", "threshold"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "operator", "higher_than"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "value", "100"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "query_period", "300"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "confirmation_period", "60"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "email", "true"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "push", "true"),
					resource.TestCheckResourceAttrSet("logtail_dashboard_alert.this", "created_at"),
				),
			},
			// Step 2 - update alert
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

				resource "logtail_dashboard_alert" "this" {
					dashboard_id        = logtail_dashboard.this.id
					chart_id            = logtail_dashboard_chart.this.id
					name                = "High Request Rate Updated"
					alert_type          = "threshold"
					operator            = "higher_than"
					value               = 200
					check_period        = 120
					query_period        = 600
					confirmation_period = 120

					email          = true
					push           = true
					call           = true
					critical_alert = true

					metadata = {
						severity = "high"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_alert.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "name", "High Request Rate Updated"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "value", "200"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "query_period", "600"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "call", "true"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "critical_alert", "true"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "metadata.severity", "high"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_dashboard_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return "1/10/100", nil
				},
			},
		},
	})
}
