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
)

func TestResourceExploration(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/explorations"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}

			// Parse and augment the response
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}

			// Add server-generated fields
			reqData["created_at"] = "2023-01-01T00:00:00Z"
			reqData["updated_at"] = "2023-01-01T00:00:00Z"

			// Add date range defaults
			if _, ok := reqData["date_range_from"]; !ok {
				reqData["date_range_from"] = "now-3h"
			}
			if _, ok := reqData["date_range_to"]; !ok {
				reqData["date_range_to"] = "now"
			}

			// Add chart defaults (name is derived from exploration name)
			if chart, ok := reqData["chart"].(map[string]interface{}); ok {
				if name, ok := reqData["name"].(string); ok {
					chart["name"] = name // API sets chart.name from exploration name
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

			data.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, respData)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodPatch && r.RequestURI == prefix+"/"+id:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			// Parse the request body
			var reqData map[string]interface{}
			if err = json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}

			// Get old data for fields not in request
			var oldData map[string]interface{}
			if err = json.Unmarshal(data.Load().([]byte), &oldData); err != nil {
				t.Fatal(err)
			}

			// Build response: use request data, keep timestamps from old
			result := make(map[string]interface{})
			result["created_at"] = oldData["created_at"]
			result["updated_at"] = "2023-01-02T00:00:00Z"

			// Copy all fields from request
			for k, v := range reqData {
				result[k] = v
			}

			// Get exploration name for chart.name
			explorationName := ""
			if name, ok := result["name"].(string); ok {
				explorationName = name
			}

			// Ensure chart has all computed fields
			if chart, ok := result["chart"].(map[string]interface{}); ok {
				chart["name"] = explorationName // API derives from exploration name
				if _, ok := chart["description"]; !ok {
					chart["description"] = ""
				}
				if _, ok := chart["settings"]; !ok {
					chart["settings"] = map[string]interface{}{}
				}
			}

			// Add query defaults
			if queries, ok := result["queries"].([]interface{}); ok {
				for i, q := range queries {
					if qMap, ok := q.(map[string]interface{}); ok {
						if _, hasID := qMap["id"]; !hasID {
							qMap["id"] = i + 1
						}
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

			patched, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			data.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, patched)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
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
			// Step 1 - create with minimal config
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
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "name", "Test Exploration"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "chart.0.chart_type", "line_chart"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "query.0.query_type", "sql_expression"),
					resource.TestCheckResourceAttrSet("logtail_exploration.this", "created_at"),
					resource.TestCheckResourceAttrSet("logtail_exploration.this", "updated_at"),
				),
			},
			// Step 2 - update
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name            = "Test Exploration Updated"
					date_range_from = "now-24h"
					date_range_to   = "now"

					chart {
						chart_type  = "line_chart"
						description = "Counts errors over time"
					}

					query {
						name       = "Main Query"
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} AND level = 'error' GROUP BY time"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "name", "Test Exploration Updated"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "date_range_from", "now-24h"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "date_range_to", "now"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "chart.0.chart_type", "line_chart"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "chart.0.name", "Test Exploration Updated"), // API derives from exploration name
					resource.TestCheckResourceAttr("logtail_exploration.this", "chart.0.description", "Counts errors over time"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "query.0.name", "Main Query"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_exploration.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestResourceExplorationWithVariables(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/explorations"
		id := "2"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}

			// Parse and augment the response
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}

			// Add server-generated fields
			reqData["created_at"] = "2023-01-01T00:00:00Z"
			reqData["updated_at"] = "2023-01-01T00:00:00Z"

			// Add date range defaults
			if _, ok := reqData["date_range_from"]; !ok {
				reqData["date_range_from"] = "now-3h"
			}
			if _, ok := reqData["date_range_to"]; !ok {
				reqData["date_range_to"] = "now"
			}

			// Add chart defaults (name is derived from exploration name)
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

			// Simulate server adding default variables
			userVars := reqData["variables"]
			defaultVars := []map[string]interface{}{
				{"name": "time", "variable_type": "datetime", "default_values": []string{}},
				{"name": "start_time", "variable_type": "datetime", "default_values": []string{}},
				{"name": "end_time", "variable_type": "datetime", "default_values": []string{}},
				{"name": "source", "variable_type": "source", "default_values": []string{}},
			}
			allVars := defaultVars
			if userVars != nil {
				if userVarsSlice, ok := userVars.([]interface{}); ok {
					for _, v := range userVarsSlice {
						if vMap, ok := v.(map[string]interface{}); ok {
							allVars = append(allVars, vMap)
						}
					}
				}
			}
			reqData["variables"] = allVars

			respData, err := json.Marshal(reqData)
			if err != nil {
				t.Fatal(err)
			}

			data.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, respData)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
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
					name = "Exploration With Variables"

					chart {
						chart_type = "line_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} AND level = {{level}} GROUP BY time"
					}

					variable {
						name          = "level"
						variable_type = "select_value"
						values        = ["error", "warning", "info"]
						default_values = ["error"]
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "name", "Exploration With Variables"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.name", "level"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.variable_type", "select_value"),
				),
			},
		},
	})
}
