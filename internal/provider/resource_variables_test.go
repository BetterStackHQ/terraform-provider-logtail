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

// The variable blocks of logtail_dashboard and logtail_exploration against an API stand-in that
// stores variables the way the real API does: a multi_select_with_sql variable is kept as
// select_with_sql with allow_multiple_values, and every variable reports the flag.

// variableEchoServer stores the resource submitted to path, reads it back with the system
// variables added and the variables normalized, and merges each PATCH into it. augment fills in
// the other server-side defaults of the resource; lastRequest receives every POST and PATCH body.
func variableEchoServer(t *testing.T, path string, augment func(map[string]interface{}), lastRequest *atomic.Value) *httptest.Server {
	var stored atomic.Value
	id := "7"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		store := func(attrs map[string]interface{}) {
			normalizeEchoedVariables(attrs)
			augment(attrs)
			data, err := json.Marshal(attrs)
			if err != nil {
				t.Fatal(err)
			}
			stored.Store(data)
		}
		respond := func(status int) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, stored.Load().([]byte))))
		}

		switch {
		case r.Method == http.MethodPost && r.RequestURI == path:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastRequest.Store(body)
			attrs := map[string]interface{}{}
			if err := json.Unmarshal(body, &attrs); err != nil {
				t.Fatal(err)
			}
			store(attrs)
			respond(http.StatusCreated)

		case r.Method == http.MethodGet && r.RequestURI == path+"/"+id:
			respond(http.StatusOK)

		case r.Method == http.MethodPatch && r.RequestURI == path+"/"+id:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastRequest.Store(body)
			attrs := map[string]interface{}{}
			if err := json.Unmarshal(stored.Load().([]byte), &attrs); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &attrs); err != nil {
				t.Fatal(err)
			}
			store(attrs)
			respond(http.StatusOK)

		case r.Method == http.MethodDelete && r.RequestURI == path+"/"+id:
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
}

func normalizeEchoedVariables(attrs map[string]interface{}) {
	variables := []interface{}{}
	seen := map[string]bool{}
	given, _ := attrs["variables"].([]interface{})
	for _, v := range given {
		variable := v.(map[string]interface{})
		if variable["variable_type"] == "multi_select_with_sql" {
			variable["variable_type"] = "select_with_sql"
			variable["allow_multiple_values"] = true
		}
		if _, ok := variable["allow_multiple_values"]; !ok {
			variable["allow_multiple_values"] = false
		}
		seen[variable["name"].(string)] = true
		variables = append(variables, variable)
	}
	for _, system := range []struct{ name, variableType string }{
		{"time", "datetime"}, {"start_time", "datetime"}, {"end_time", "datetime"}, {"source", "source"},
	} {
		if seen[system.name] {
			continue
		}
		variables = append(variables, map[string]interface{}{
			"name": system.name, "variable_type": system.variableType, "values": []string{}, "default_values": []string{}, "allow_multiple_values": false,
		})
	}
	attrs["variables"] = variables
}

func dashboardEchoDefaults(attrs map[string]interface{}) {
	attrs["created_at"] = "2023-01-01T00:00:00Z"
	attrs["updated_at"] = "2023-01-01T00:00:00Z"
	for key, value := range map[string]interface{}{"date_range_from": "now-3h", "date_range_to": "now", "refresh_interval": 0} {
		if _, ok := attrs[key]; !ok {
			attrs[key] = value
		}
	}
}

func explorationEchoDefaults(attrs map[string]interface{}) {
	attrs["created_at"] = "2023-01-01T00:00:00Z"
	attrs["updated_at"] = "2023-01-01T00:00:00Z"
	for key, value := range map[string]interface{}{"date_range_from": "now-3h", "date_range_to": "now"} {
		if _, ok := attrs[key]; !ok {
			attrs[key] = value
		}
	}
	if chart, ok := attrs["chart"].(map[string]interface{}); ok {
		chart["name"] = attrs["name"]
		for key, value := range map[string]interface{}{"description": "", "settings": map[string]interface{}{}} {
			if _, ok := chart[key]; !ok {
				chart[key] = value
			}
		}
	}
	if queries, ok := attrs["queries"].([]interface{}); ok {
		for i, q := range queries {
			query := q.(map[string]interface{})
			query["id"] = i + 1
			for key, value := range map[string]interface{}{"source_variable": "source", "name": "", "where_condition": "", "static_text": ""} {
				if _, ok := query[key]; !ok {
					query[key] = value
				}
			}
		}
	}
}

func checkLastRequestContains(lastRequest *atomic.Value, fragment string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		body, _ := lastRequest.Load().([]byte)
		if !strings.Contains(string(body), fragment) {
			return fmt.Errorf("expected the last request to contain %s, got %s", fragment, body)
		}
		return nil
	}
}

func variableTestProviderFactories(server *httptest.Server) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"logtail": func() (*schema.Provider, error) {
			return New(WithURL(server.URL)), nil
		},
	}
}

// A multi_select_with_sql variable comes back from the API as select_with_sql with
// allow_multiple_values, which is what state holds. The configured spelling must not show up as a
// change on every plan, and an update of anything else resends the variable as the API stores it.
func TestResourceDashboardMultiSelectWithSQLVariable(t *testing.T) {
	var lastRequest atomic.Value
	server := variableEchoServer(t, "/api/v2/dashboards", dashboardEchoDefaults, &lastRequest)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: variableTestProviderFactories(server),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Multi-select variables"

					variable {
						name           = "hosts"
						variable_type  = "multi_select_with_sql"
						sql_definition = "label('host')"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.#", "1"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.name", "hosts"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.allow_multiple_values", "true"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.sql_definition", "label('host')"),
				),
			},
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Multi-select variables, renamed"

					variable {
						name           = "hosts"
						variable_type  = "multi_select_with_sql"
						sql_definition = "label('host')"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", "Multi-select variables, renamed"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.allow_multiple_values", "true"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"label('host')","allow_multiple_values":true`),
				),
			},
		},
	})
}

func TestResourceDashboardAllowMultipleValues(t *testing.T) {
	var lastRequest atomic.Value
	server := variableEchoServer(t, "/api/v2/dashboards", dashboardEchoDefaults, &lastRequest)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: variableTestProviderFactories(server),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Multi-select variables"

					variable {
						name                  = "environment"
						variable_type         = "select_value"
						default_values        = ["production", "staging"]
						values                = ["production", "staging"]
						allow_multiple_values = true
					}

					variable {
						name                  = "hosts"
						variable_type         = "select_with_sql"
						sql_definition        = "label('host')"
						allow_multiple_values = true
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.#", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.variable_type", "select_value"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.allow_multiple_values", "true"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.1.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.1.allow_multiple_values", "true"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"label('host')","allow_multiple_values":true`),
				),
			},
			// Turning the flag off has to reach the API as an explicit false.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Multi-select variables"

					variable {
						name                  = "environment"
						variable_type         = "select_value"
						default_values        = ["production", "staging"]
						values                = ["production"]
						allow_multiple_values = false
					}

					variable {
						name           = "hosts"
						variable_type  = "select_with_sql"
						sql_definition = "label('host')"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.allow_multiple_values", "false"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.1.allow_multiple_values", "false"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"label('host')","allow_multiple_values":false`),
				),
			},
		},
	})
}

func TestResourceExplorationMultiSelectWithSQLVariable(t *testing.T) {
	var lastRequest atomic.Value
	server := variableEchoServer(t, "/api/v2/explorations", explorationEchoDefaults, &lastRequest)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: variableTestProviderFactories(server),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Multi-select variables"

					chart {
						chart_type = "bar_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} [[ AND match(JSONExtractString(raw, 'host'), {{hosts}}) ]] GROUP BY time"
					}

					variable {
						name           = "hosts"
						variable_type  = "multi_select_with_sql"
						sql_definition = "JSONExtractString(raw, 'host')"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.#", "1"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.name", "hosts"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.allow_multiple_values", "true"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.sql_definition", "JSONExtractString(raw, 'host')"),
				),
			},
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Multi-select variables, renamed"

					chart {
						chart_type = "bar_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} [[ AND match(JSONExtractString(raw, 'host'), {{hosts}}) ]] GROUP BY time"
					}

					variable {
						name           = "hosts"
						variable_type  = "multi_select_with_sql"
						sql_definition = "JSONExtractString(raw, 'host')"
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_exploration.this", "name", "Multi-select variables, renamed"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.allow_multiple_values", "true"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"JSONExtractString(raw, 'host')","allow_multiple_values":true`),
				),
			},
			// An import reads the variable back in the form the API stores, and that matches a
			// configuration in the legacy spelling without a change.
			{
				ResourceName:      "logtail_exploration.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "7",
			},
		},
	})
}

func TestResourceExplorationAllowMultipleValues(t *testing.T) {
	var lastRequest atomic.Value
	server := variableEchoServer(t, "/api/v2/explorations", explorationEchoDefaults, &lastRequest)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: variableTestProviderFactories(server),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Multi-select variables"

					chart {
						chart_type = "bar_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} [[ AND match(JSONExtractString(raw, 'host'), {{hosts}}) ]] GROUP BY time"
					}

					variable {
						name                  = "hosts"
						variable_type         = "select_with_sql"
						sql_definition        = "JSONExtractString(raw, 'host')"
						allow_multiple_values = true
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.#", "1"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.variable_type", "select_with_sql"),
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.allow_multiple_values", "true"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"JSONExtractString(raw, 'host')","allow_multiple_values":true`),
				),
			},
			// The flag is part of what an import reads back.
			{
				ResourceName:      "logtail_exploration.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "7",
			},
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration" "this" {
					name = "Multi-select variables"

					chart {
						chart_type = "bar_chart"
					}

					query {
						query_type = "sql_expression"
						sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} [[ AND match(JSONExtractString(raw, 'host'), {{hosts}}) ]] GROUP BY time"
					}

					variable {
						name                  = "hosts"
						variable_type         = "select_with_sql"
						sql_definition        = "JSONExtractString(raw, 'host')"
						allow_multiple_values = false
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_exploration.this", "variable.0.allow_multiple_values", "false"),
					checkLastRequestContains(&lastRequest, `"name":"hosts","variable_type":"select_with_sql","sql_definition":"JSONExtractString(raw, 'host')","allow_multiple_values":false`),
				),
			},
		},
	})
}
