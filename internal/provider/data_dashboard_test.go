package provider

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceDashboard(t *testing.T) {
	var id atomic.Value
	id.Store("123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/123":
			_, _ = w.Write([]byte(`{"data":{"id":"123","type":"dashboard","attributes":{"name":"Unique Dashboard A","team_id":123,"created_at":"2025-01-20T01:00:00.000Z","updated_at":"2025-01-20T01:30:00.000Z"}}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/123/export":
			_, _ = w.Write([]byte(`{"id":"123","name":"Unique Dashboard A","data":{"refresh_interval":30,"date_range_from":"now-1h","date_range_to":"now","charts":[],"sections":[]}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/234":
			_, _ = w.Write([]byte(`{"data":{"id":"234","type":"dashboard","attributes":{"name":"Duplicate Dashboard A","team_id":123,"created_at":"2025-01-20T02:00:00.000Z","updated_at":"2025-01-20T02:30:00.000Z"}}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/234/export":
			_, _ = w.Write([]byte(`{"id":"234","name":"Duplicate Dashboard A","data":{"refresh_interval":30,"date_range_from":"now-1h","date_range_to":"now","charts":[],"sections":[]}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/789":
			_, _ = w.Write([]byte(`{"data":{"id":"789","type":"dashboard","attributes":{"name":"Dashboard with broken export","team_id":123,"created_at":"2025-01-20T07:00:00.000Z","updated_at":"2025-01-20T07:30:00.000Z"}}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/789/export":
			// Simulate broken export - return 500 error
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Export failed"}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards?page=1":
			// Page 1: Unique A, Duplicate A, Duplicate A, Duplicate B
			_, _ = w.Write([]byte(`{"data":[
				{"id":"123","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Unique Dashboard A","created_at":"2025-01-20T01:00:00.000Z","updated_at":"2025-01-20T01:30:00.000Z"}},
				{"id":"234","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Duplicate Dashboard A","created_at":"2025-01-20T02:00:00.000Z","updated_at":"2025-01-20T02:30:00.000Z"}},
				{"id":"345","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Duplicate Dashboard A","created_at":"2025-01-20T03:00:00.000Z","updated_at":"2025-01-20T03:30:00.000Z"}},
				{"id":"456","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Duplicate Dashboard B","created_at":"2025-01-20T04:00:00.000Z","updated_at":"2025-01-20T04:30:00.000Z"}}
			],"pagination":{"next":"https://test.com/api/v2/dashboards?page=2"}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards?page=2":
			// Page 2: Unique B, Duplicate B, Broken Export
			_, _ = w.Write([]byte(`{"data":[
				{"id":"567","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Unique Dashboard B","created_at":"2025-01-20T05:00:00.000Z","updated_at":"2025-01-20T05:30:00.000Z"}},
				{"id":"678","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Duplicate Dashboard B","created_at":"2025-01-20T06:00:00.000Z","updated_at":"2025-01-20T06:30:00.000Z"}},
				{"id":"789","type":"dashboard","attributes":{"team_id":123,"team_name":"test-team","name":"Dashboard with broken export","created_at":"2025-01-20T07:00:00.000Z","updated_at":"2025-01-20T07:30:00.000Z"}}
			],"pagination":{"next":null}}`))
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
			// Step 1 - lookup by ID
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "by_id" {
					id = "123"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "name", "Unique Dashboard A"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "team_id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "data", `{"charts":[],"date_range_from":"now-1h","date_range_to":"now","refresh_interval":30,"sections":[]}`),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "created_at", "2025-01-20T01:00:00.000Z"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_id", "updated_at", "2025-01-20T01:30:00.000Z"),
				),
				PreConfig: func() {
					t.Log("step 1 - lookup by ID")
				},
			},
			// Step 2 - lookup by name
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "by_name" {
					name = "Unique Dashboard A"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "name", "Unique Dashboard A"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "team_id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "data", `{"charts":[],"date_range_from":"now-1h","date_range_to":"now","refresh_interval":30,"sections":[]}`),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "created_at", "2025-01-20T01:00:00.000Z"),
					resource.TestCheckResourceAttr("data.logtail_dashboard.by_name", "updated_at", "2025-01-20T01:30:00.000Z"),
				),
				PreConfig: func() {
					t.Log("step 2 - lookup by name")
				},
			},
			// Step 3 - lookup by duplicate name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "duplicate" {
					name = "Duplicate Dashboard A"
				}
				`,
				ExpectError: regexp.MustCompile(`multiple dashboards found with the name "Duplicate Dashboard A" - use ID lookup instead, available dashboard IDs: 234, 345`),
				PreConfig: func() {
					t.Log("step 3 - lookup by duplicate name (should error)")
				},
			},
			// Step 4 - lookup by non-existent name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "nonexistent" {
					name = "Non-Existent Dashboard"
				}
				`,
				ExpectError: regexp.MustCompile(`no dashboard found with name "Non-Existent Dashboard"`),
				PreConfig: func() {
					t.Log("step 4 - lookup by non-existent name (should error)")
				},
			},
			// Step 5 - lookup by ID with mismatched name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "mismatch" {
					id = "123"
					name = "Expected Name"
				}
				`,
				ExpectError: regexp.MustCompile(`dashboard with ID 123 has name "Unique Dashboard A", but requested name is "Expected Name"`),
				PreConfig: func() {
					t.Log("step 5 - lookup by ID with mismatched name (should error)")
				},
			},
			// Step 6 - lookup by ID with broken export (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard" "broken_export" {
					id = "789"
				}
				`,
				ExpectError: regexp.MustCompile(`dashboard "Dashboard with broken export" was found but couldn't be exported`),
				PreConfig: func() {
					t.Log("step 6 - lookup by ID with broken export (should error)")
				},
			},
		},
	})
}
