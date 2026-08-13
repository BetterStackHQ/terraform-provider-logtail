package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// additional_conditions contract: the array holds conditions 2..5 combined with the main
// condition by logical AND; each object carries only the attributes the user set (a string
// condition must not leak "value": 0), while the API echoes every condition normalized to
// all six keys with nulls and empty arrays. Sending [] clears all conditions; an absent key
// leaves them untouched, so removing every block must still send []. The mock mirrors that.
func TestResourceDashboardAlertAdditionalConditions(t *testing.T) {
	var alertData atomic.Value

	// normalizeConditions mirrors ChartAlert::Condition#to_h: all six keys, wire nulls.
	normalizeConditions := func(reqData map[string]interface{}) {
		raw, ok := reqData["additional_conditions"].([]interface{})
		if !ok {
			return
		}
		for i, item := range raw {
			cond, ok := item.(map[string]interface{})
			if !ok {
				t.Fatalf("condition %d is not an object: %v", i, item)
			}
			for _, key := range []string{"alert_type", "operator", "value", "string_value"} {
				if _, ok := cond[key]; !ok {
					cond[key] = nil
				}
			}
			for _, key := range []string{"series_names", "series_names_except"} {
				if _, ok := cond[key]; !ok {
					cond[key] = []string{}
				}
			}
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		alertPrefix := "/api/v2/dashboards/1/charts/10/alerts"
		alertID := "100"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == alertPrefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				t.Fatal(err)
			}
			// The string condition set no numeric value; a leaked zero would flip the
			// API into numeric comparison, so the wire shape is asserted exactly.
			conds, ok := reqData["additional_conditions"].([]interface{})
			if !ok || len(conds) != 1 {
				t.Fatalf("expected exactly one additional condition, got %v", reqData["additional_conditions"])
			}
			cond := conds[0].(map[string]interface{})
			if len(cond) != 3 || cond["alert_type"] != "threshold" || cond["operator"] != "lower_than" || cond["value"] != float64(10000) {
				t.Fatalf("unexpected condition payload: %v", cond)
			}
			normalizeConditions(reqData)
			reqData["series_names"] = []string{}
			reqData["series_names_except"] = []string{}
			reqData["source_platforms"] = []string{}
			if _, ok := reqData["on_missing_data"]; !ok {
				reqData["on_missing_data"] = "treat_as_zero"
			}
			if _, ok := reqData["source_mode"]; !ok {
				reqData["source_mode"] = "source_variable"
			}
			if _, ok := reqData["paused"]; !ok {
				reqData["paused"] = false
			}
			if _, ok := reqData["recovery_period"]; !ok {
				reqData["recovery_period"] = 60
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
			var patchReq map[string]interface{}
			if err = json.Unmarshal(body, &patchReq); err != nil {
				t.Fatal(err)
			}
			normalizeConditions(patchReq)
			patch := make(map[string]interface{})
			if err = json.Unmarshal(alertData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			for k, v := range patchReq {
				patch[k] = v
			}
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

	configWithConditions := func(conditionBlocks string) string {
		return fmt.Sprintf(`
		provider "logtail" {
			api_token = "foo"
		}

		resource "logtail_dashboard_alert" "this" {
			dashboard_id = "1"
			chart_id     = "10"
			name         = "Band Alert"
			alert_type   = "threshold"
			operator     = "higher_than"
			value        = 100
			check_period = 300
			%s
		}
		`, conditionBlocks)
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with one extra condition (a band alert: > 100 and < 10000).
			{
				Config: configWithConditions(`
					additional_conditions {
						alert_type = "threshold"
						operator   = "lower_than"
						value      = 10000
					}
				`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.#", "1"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.0.alert_type", "threshold"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.0.operator", "lower_than"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.0.value", "10000"),
				),
			},
			// Step 2 - the normalized echo (null string_value, empty series arrays) is not drift.
			{
				Config: configWithConditions(`
					additional_conditions {
						alert_type = "threshold"
						operator   = "lower_than"
						value      = 10000
					}
				`),
				PlanOnly: true,
			},
			// Step 3 - grow to two conditions, the second scoped to specific series.
			{
				Config: configWithConditions(`
					additional_conditions {
						alert_type = "threshold"
						operator   = "lower_than"
						value      = 10000
					}
					additional_conditions {
						alert_type   = "relative"
						operator     = "increases_by"
						value        = 50
						series_names = ["production"]
					}
				`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.#", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.1.alert_type", "relative"),
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.1.series_names.0", "production"),
				),
			},
			// Step 4 - import round-trips the normalized conditions.
			{
				ResourceName:      "logtail_dashboard_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "1/10/100",
			},
			// Step 5 - removing every block clears the conditions ([] on the wire;
			// an absent key would leave them untouched server-side).
			{
				Config: configWithConditions(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "additional_conditions.#", "0"),
				),
			},
			// Step 6 - both series scopes in one condition are rejected in the plan
			// (the API would silently keep only series_names_except).
			{
				PlanOnly: true,
				Config: configWithConditions(`
					additional_conditions {
						alert_type          = "threshold"
						operator            = "lower_than"
						value               = 10000
						series_names        = ["production"]
						series_names_except = ["staging"]
					}
				`),
				ExpectError: regexp.MustCompile("cannot set both series_names and series_names_except"),
			},
		},
	})
}
