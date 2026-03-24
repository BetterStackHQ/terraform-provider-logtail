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

func TestResourceDashboard(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	var currentName atomic.Value
	var currentGroupID atomic.Value
	var readCount atomic.Value
	readCount.Store(0)
	id.Store(0)
	currentName.Store("Test Dashboard")
	currentGroupID.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		dashboardAttrs := func() string {
			groupID := currentGroupID.Load().(int)
			groupField := ""
			if groupID > 0 {
				groupField = fmt.Sprintf(`,"dashboard_group_id":%d`, groupID)
			}
			return fmt.Sprintf(`"name":"%s","team_id":123%s,"created_at":"2025-01-20T15:00:00.000Z"`, currentName.Load().(string), groupField)
		}

		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/api/v2/dashboards/import":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)

			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			currentId := id.Load().(int)

			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","type":"dashboard","attributes":{%s,"updated_at":"2025-01-20T15:00:00.000Z"}}}`, currentId, dashboardAttrs())))

		case r.Method == http.MethodPatch && strings.HasPrefix(r.RequestURI, "/api/v2/dashboards/"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var patch map[string]interface{}
			if err := json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			if name, ok := patch["name"].(string); ok {
				currentName.Store(name)
			}
			if gid, ok := patch["dashboard_group_id"].(float64); ok {
				currentGroupID.Store(int(gid))
			}
			currentId := id.Load().(int)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","type":"dashboard","attributes":{%s,"updated_at":"2025-01-20T16:00:00.000Z"}}}`, currentId, dashboardAttrs())))

		case r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/api/v2/dashboards/%d", id.Load()):
			currentId := id.Load().(int)
			updatedAt := "2025-01-20T15:00:00.000Z"

			readCount.Store(readCount.Load().(int) + 1)
			readCountInt := readCount.Load().(int)

			if readCountInt >= 3 {
				updatedAt = "2025-01-20T15:30:00.000Z"
			}

			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","type":"dashboard","attributes":{%s,"updated_at":"%s"}}}`, currentId, dashboardAttrs(), updatedAt)))

		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf("/api/v2/dashboards/%d", id.Load()):
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
			currentName.Store("Test Dashboard")
			currentGroupID.Store(0)

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var name = "Test Dashboard"
	var updatedName = "Renamed Dashboard"
	var dashboardData = `{"refresh_interval":30,"date_range_from":"now-1h","date_range_to":"now","charts":[],"sections":[]}`
	var updatedDashboardData = `{"refresh_interval":60,"date_range_from":"now-2h","date_range_to":"now","charts":[],"sections":[]}`

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create dashboard successfully
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "%s"
					data = %q
				}
				`, name, dashboardData),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", name),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "data", dashboardData),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "team_id", "123"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "created_at", "2025-01-20T15:00:00.000Z"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "updated_at", "2025-01-20T15:00:00.000Z"),
				),
			},
			// Step 2 - change data (should recreate due to ForceNew)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "%s"
					data = %q
				}
				`, name, updatedDashboardData),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", "Test Dashboard"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "data", updatedDashboardData),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "team_id", "123"),
				),
			},
			// Step 3 - test updated_at changes are reflected from API
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "%s"
					data = %q
				}
				`, name, updatedDashboardData),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "updated_at", "2025-01-20T15:30:00.000Z"),
				),
			},
			// Step 4 - rename dashboard in import mode (should update in-place, not recreate)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "%s"
					data = %q
				}
				`, updatedName, updatedDashboardData),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", updatedName),
				),
			},
			// Step 5 - set dashboard_group_id in import mode (should update in-place)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name               = "%s"
					data               = %q
					dashboard_group_id = 42
				}
				`, updatedName, updatedDashboardData),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "2"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", updatedName),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "dashboard_group_id", "42"),
				),
			},
		},
	})
}

func TestResourceDashboardCRUDMode(t *testing.T) {
	var dashboardData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		dashboardID := "1"

		switch {
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

		case r.Method == http.MethodPatch && r.RequestURI == "/api/v2/dashboards/"+dashboardID:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(dashboardData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patch["updated_at"] = "2023-01-02T00:00:00Z"
			patched, _ := json.Marshal(patch)
			dashboardData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, dashboardID, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == "/api/v2/dashboards/"+dashboardID:
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
			// Step 1 - create dashboard in CRUD mode
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name             = "My Dashboard"
					date_range_from  = "now-3h"
					date_range_to    = "now"
					refresh_interval = 30

					variable {
						name          = "env"
						variable_type = "string"
						values        = ["production", "staging"]
						default_values = ["production"]
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", "My Dashboard"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "date_range_from", "now-3h"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "date_range_to", "now"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "refresh_interval", "30"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.name", "env"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "variable.0.variable_type", "string"),
					resource.TestCheckResourceAttrSet("logtail_dashboard.this", "created_at"),
				),
			},
			// Step 2 - update dashboard
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name             = "My Dashboard Updated"
					date_range_from  = "now-24h"
					date_range_to    = "now"
					refresh_interval = 60

					variable {
						name          = "env"
						variable_type = "string"
						values        = ["production", "staging", "dev"]
						default_values = ["production"]
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_dashboard.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", "My Dashboard Updated"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "date_range_from", "now-24h"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "refresh_interval", "60"),
				),
			},
			// Step 3 - import
			{
				ResourceName:            "logtail_dashboard.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"variable"},
			},
		},
	})
}

func TestResourceDashboardDualModeValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL("http://localhost")), nil
			},
		},
		Steps: []resource.TestStep{
			// Test that mixing data with CRUD fields produces error
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name             = "Test"
					data             = "{\"charts\":[]}"
					refresh_interval = 30
				}
				`,
				ExpectError: regexp.MustCompile(`cannot use 'data' \(import mode\) together with individual dashboard fields`),
			},
		},
	})
}
