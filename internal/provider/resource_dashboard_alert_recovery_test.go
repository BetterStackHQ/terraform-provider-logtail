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
)

// recovery_period contract: an explicit null is the Never mode, which the API stores and
// serves as null; 0 recovers immediately; an absent key defaults to 60 on create; negative
// values are rejected. HCL cannot distinguish null from an unset attribute, so Terraform
// represents Never as -1 and must translate it to null on the wire (and back on read).
// The mock mirrors the API contract.
func TestResourceDashboardAlertRecoveryPeriodNever(t *testing.T) {
	var alertData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		alertPrefix := "/api/v2/dashboards/1/charts/10/alerts"
		alertID := "100"

		// Negative values are rejected like the real API; null passes through as Never.
		// Returns true when the request was rejected.
		rejectNegativeRecoveryPeriod := func(reqData map[string]interface{}) bool {
			if v, ok := reqData["recovery_period"]; ok && v != nil {
				if f, isNum := v.(float64); isNum && f < 0 {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"errors":{"recovery_period":["must be greater than or equal to 0"]}}`))
					return true
				}
			}
			return false
		}

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
			if rejectNegativeRecoveryPeriod(reqData) {
				return
			}
			if _, ok := reqData["recovery_period"]; !ok {
				reqData["recovery_period"] = 60
			}
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
			if rejectNegativeRecoveryPeriod(patchReq) {
				return
			}
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

	configWithRecoveryPeriod := func(recoveryPeriodLine string) string {
		return fmt.Sprintf(`
		provider "logtail" {
			api_token = "foo"
		}

		resource "logtail_dashboard_alert" "this" {
			dashboard_id = "1"
			chart_id     = "10"
			name         = "Recovery Period Alert"
			alert_type   = "threshold"
			operator     = "higher_than"
			value        = 100
			check_period = 300
			%s
		}
		`, recoveryPeriodLine)
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create without recovery_period: the API default of 60 is read back.
			{
				Config: configWithRecoveryPeriod(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "recovery_period", "60"),
				),
			},
			// Step 2 - switch to Never (-1 in Terraform, null on the wire).
			{
				Config: configWithRecoveryPeriod("recovery_period = -1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "recovery_period", "-1"),
				),
			},
			// Step 3 - no changes, the null read back as -1 must not produce a diff.
			{
				Config:   configWithRecoveryPeriod("recovery_period = -1"),
				PlanOnly: true,
			},
			// Step 4 - switch to a regular recovery period.
			{
				Config: configWithRecoveryPeriod("recovery_period = 300"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "recovery_period", "300"),
				),
			},
			// Step 5 - and back to Never.
			{
				Config: configWithRecoveryPeriod("recovery_period = -1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard_alert.this", "recovery_period", "-1"),
				),
			},
			// Step 6 - import reads the null back as -1.
			{
				ResourceName:      "logtail_dashboard_alert.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "1/10/100",
			},
		},
	})
}
