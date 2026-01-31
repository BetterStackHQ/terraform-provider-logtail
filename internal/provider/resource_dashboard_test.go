package provider

import (
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
	var readCount atomic.Value
	readCount.Store(0)
	id.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodPost && r.RequestURI == "/api/v2/dashboards/import":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)

			// Check if this is a test for import failure
			bodyStr := string(body)
			if strings.Contains(bodyStr, `"name":"Fail Import"`) {
				// Simulate import API failure
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"Import failed due to server error"}`))
				return
			}

			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			currentId := id.Load().(int)

			// Different responses based on ID for testing
			name := "Test Dashboard"
			updatedAt := "2025-01-20T15:00:00.000Z"
			if currentId == 2 {
				name = "Updated Dashboard Name"
			} else if currentId == 3 {
				name = "Recreated Dashboard"
				updatedAt = "2025-01-20T16:00:00.000Z"
			}

			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","type":"dashboard","attributes":{"name":"%s","team_id":123,"created_at":"2025-01-20T15:00:00.000Z","updated_at":"%s"}}}`, currentId, name, updatedAt)))

		case r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/api/v2/dashboards/%d", id.Load()):
			currentId := id.Load().(int)
			name := "Test Dashboard"
			updatedAt := "2025-01-20T15:00:00.000Z"

			readCount.Store(readCount.Load().(int) + 1)
			readCountInt := readCount.Load().(int)

			// Simulate updated_at changing in API response for testing
			if readCountInt >= 3 { // After step 3 (data change recreation)
				updatedAt = "2025-01-20T15:30:00.000Z"
			}

			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","type":"dashboard","attributes":{"name":"%s","team_id":123,"created_at":"2025-01-20T15:00:00.000Z","updated_at":"%s"}}}`, currentId, name, updatedAt)))

		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf("/api/v2/dashboards/%d", id.Load()):
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var name = "Test Dashboard"
	var updatedName = "Updated Dashboard Name"
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
				PreConfig: func() {
					t.Log("step 1 - create dashboard successfully")
				},
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
					resource.TestCheckResourceAttr("logtail_dashboard.this", "created_at", "2025-01-20T15:00:00.000Z"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "updated_at", "2025-01-20T15:00:00.000Z"),
				),
				PreConfig: func() {
					t.Log("step 2 - change data (should recreate due to ForceNew)")
				},
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
					resource.TestCheckResourceAttr("logtail_dashboard.this", "name", "Test Dashboard"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "data", updatedDashboardData),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "team_id", "123"),
					resource.TestCheckResourceAttr("logtail_dashboard.this", "created_at", "2025-01-20T15:00:00.000Z"),
					// This should show the updated timestamp from the API
					resource.TestCheckResourceAttr("logtail_dashboard.this", "updated_at", "2025-01-20T15:30:00.000Z"),
				),
				PreConfig: func() {
					t.Log("step 3 - test updated_at changes are reflected from API")
				},
			},
			// Step 4 - change name (should fail since updates not supported)
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
				ExpectError: regexp.MustCompile(`dashboard updates are not supported - please rename the dashboard in Better Stack`),
				PreConfig: func() {
					t.Log("step 4 - change name (should fail since updates not supported)")
				},
			},
		},
	})
}
