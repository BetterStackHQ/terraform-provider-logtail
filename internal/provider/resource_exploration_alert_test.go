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

func TestResourceExplorationAlert(t *testing.T) {
	var explorationData atomic.Value
	var alertData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		explorationID := "1"
		alertID := "10"
		explorationPrefix := "/api/v2/explorations"
		alertPrefix := fmt.Sprintf("/api/v2/explorations/%s/alerts", explorationID)

		switch {
		// Exploration CRUD
		case r.Method == http.MethodPost && r.RequestURI == explorationPrefix:
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
			// Add date range defaults
			if _, ok := reqData["date_range_from"]; !ok {
				reqData["date_range_from"] = "now-3h"
			}
			if _, ok := reqData["date_range_to"]; !ok {
				reqData["date_range_to"] = "now"
			}
			// Add chart defaults
			if chart, ok := reqData["chart"].(map[string]interface{}); ok {
				if name, ok := reqData["name"].(string); ok {
					chart["name"] = name
				}
				if _, ok := chart["description"]; !ok {
					chart["description"] = ""
				}
				if _, ok := chart["settings"]; !ok {
					chart["settings"] = map[string]interface{}{}
				}
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
			respData, err := json.Marshal(reqData)
			if err != nil {
				t.Fatal(err)
			}
			explorationData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, explorationID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == explorationPrefix+"/"+explorationID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, explorationID, explorationData.Load().([]byte))))

		case r.Method == http.MethodDelete && r.RequestURI == explorationPrefix+"/"+explorationID:
			w.WriteHeader(http.StatusNoContent)
			explorationData.Store([]byte(nil))

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
			// Add API-computed fields
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
			if _, ok := reqData["incident_per_series"]; !ok {
				reqData["incident_per_series"] = false
			}
			if _, ok := reqData["anomaly_trigger"]; !ok {
				reqData["anomaly_trigger"] = "any"
			}
			respData, err := json.Marshal(reqData)
			if err != nil {
				t.Fatal(err)
			}
			alertData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == alertPrefix+"/"+alertID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, alertData.Load().([]byte))))

		case r.Method == http.MethodPatch && r.RequestURI == alertPrefix+"/"+alertID:
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
			patched, err := json.Marshal(patch)
			if err != nil {
				t.Fatal(err)
			}
			alertData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == alertPrefix+"/"+alertID:
			w.WriteHeader(http.StatusNoContent)
			alertData.Store([]byte(nil))

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

				resource "logtail_exploration" "this" {
					name = "Test Exploration"

					chart {
						chart_type = "line_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
					}
				}

				resource "logtail_exploration_alert" "this" {
					exploration_id      = logtail_exploration.this.id
					name                = "Test Alert"
					alert_type          = "threshold"
					operator            = "higher_than"
					value               = 100
					query_period        = 300
					confirmation_period = 60
					recovery_period     = 300

					email = true
					push  = true
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration_alert.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "name", "Test Alert"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "alert_type", "threshold"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "operator", "higher_than"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "value", "100"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "query_period", "300"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "confirmation_period", "60"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "email", "true"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "push", "true"),
					resource.TestCheckResourceAttrSet("logtail_exploration_alert.this", "created_at"),
				),
			},
			// Step 2 - update alert
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Test Exploration"

					chart {
						chart_type = "line_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
					}
				}

				resource "logtail_exploration_alert" "this" {
					exploration_id      = logtail_exploration.this.id
					name                = "Test Alert Updated"
					alert_type          = "threshold"
					operator            = "higher_than"
					value               = 200
					query_period        = 600
					confirmation_period = 120
					recovery_period     = 600

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
					resource.TestCheckResourceAttrSet("logtail_exploration_alert.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "name", "Test Alert Updated"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "value", "200"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "query_period", "600"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "call", "true"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "critical_alert", "true"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "metadata.severity", "high"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_exploration_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					// Return composite ID format: exploration_id/alert_id
					return "1/10", nil
				},
			},
		},
	})
}

func TestResourceExplorationAlertWithEscalationTarget(t *testing.T) {
	var explorationData atomic.Value
	var alertData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		explorationID := "2"
		alertID := "20"
		explorationPrefix := "/api/v2/explorations"
		alertPrefix := fmt.Sprintf("/api/v2/explorations/%s/alerts", explorationID)

		switch {
		case r.Method == http.MethodPost && r.RequestURI == explorationPrefix:
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
			// Add date range defaults
			if _, ok := reqData["date_range_from"]; !ok {
				reqData["date_range_from"] = "now-3h"
			}
			if _, ok := reqData["date_range_to"]; !ok {
				reqData["date_range_to"] = "now"
			}
			// Add chart defaults
			if chart, ok := reqData["chart"].(map[string]interface{}); ok {
				if name, ok := reqData["name"].(string); ok {
					chart["name"] = name
				}
				if _, ok := chart["description"]; !ok {
					chart["description"] = ""
				}
				if _, ok := chart["settings"]; !ok {
					chart["settings"] = map[string]interface{}{}
				}
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
			respData, err := json.Marshal(reqData)
			if err != nil {
				t.Fatal(err)
			}
			explorationData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, explorationID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == explorationPrefix+"/"+explorationID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, explorationID, explorationData.Load().([]byte))))

		case r.Method == http.MethodDelete && r.RequestURI == explorationPrefix+"/"+explorationID:
			w.WriteHeader(http.StatusNoContent)
			explorationData.Store([]byte(nil))

		case r.Method == http.MethodPost && r.RequestURI == alertPrefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}
			// Add API-computed fields
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
			if _, ok := reqData["incident_per_series"]; !ok {
				reqData["incident_per_series"] = false
			}
			if _, ok := reqData["anomaly_trigger"]; !ok {
				reqData["anomaly_trigger"] = "any"
			}
			if _, ok := reqData["query_period"]; !ok {
				reqData["query_period"] = 60
			}
			respData, err := json.Marshal(reqData)
			if err != nil {
				t.Fatal(err)
			}
			alertData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, respData)))

		case r.Method == http.MethodGet && strings.HasPrefix(r.RequestURI, alertPrefix+"/"):
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, alertID, alertData.Load().([]byte))))

		case r.Method == http.MethodDelete && strings.HasPrefix(r.RequestURI, alertPrefix+"/"):
			w.WriteHeader(http.StatusNoContent)
			alertData.Store([]byte(nil))

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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Test Exploration 2"

					chart {
						chart_type = "line_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
					}
				}

				resource "logtail_exploration_alert" "this" {
					exploration_id      = logtail_exploration.this.id
					name                = "Alert with Escalation Policy"
					alert_type          = "threshold"
					operator            = "higher_than"
					value               = 50
					confirmation_period = 0

					escalation_target {
						policy_id = 123
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration_alert.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "name", "Alert with Escalation Policy"),
					resource.TestCheckResourceAttr("logtail_exploration_alert.this", "escalation_target.0.policy_id", "123"),
				),
			},
		},
	})
}
