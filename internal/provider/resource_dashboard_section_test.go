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

func TestResourceDashboardSection(t *testing.T) {
	var dashboardData atomic.Value
	var sectionData atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		dashboardID := "1"
		sectionID := "10"

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

		// Section CRUD
		case r.Method == http.MethodPost && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/sections":
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
			if _, ok := reqData["collapsed"]; !ok {
				reqData["collapsed"] = false
			}
			respData, _ := json.Marshal(reqData)
			sectionData.Store(respData)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, sectionID, respData)))

		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/sections/"+sectionID:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, sectionID, sectionData.Load().([]byte))))

		case r.Method == http.MethodPatch && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/sections/"+sectionID:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(sectionData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patch["updated_at"] = "2023-01-02T00:00:00Z"
			patched, _ := json.Marshal(patch)
			sectionData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, sectionID, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == "/api/v2/dashboards/"+dashboardID+"/sections/"+sectionID:
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
			// Step 1 - create section
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Test Dashboard"
				}

				resource "logtail_dashboard_section" "this" {
					dashboard_id = logtail_dashboard.this.id
					name         = "Performance"
					y            = 8
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_section.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "name", "Performance"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "y", "8"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "collapsed", "false"),
					resource.TestCheckResourceAttrSet("logtail_dashboard_section.this", "created_at"),
				),
			},
			// Step 2 - update section
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_dashboard" "this" {
					name = "Test Dashboard"
				}

				resource "logtail_dashboard_section" "this" {
					dashboard_id = logtail_dashboard.this.id
					name         = "Performance Updated"
					y            = 12
					collapsed    = true
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_dashboard_section.this", "id"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "name", "Performance Updated"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "y", "12"),
					resource.TestCheckResourceAttr("logtail_dashboard_section.this", "collapsed", "true"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_dashboard_section.this",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return "1/10", nil
				},
			},
		},
	})
}
