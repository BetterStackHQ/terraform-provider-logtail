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

func TestDataSourceDashboardTemplate(t *testing.T) {
	var id atomic.Value
	id.Store("123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/123/export":
			_, _ = w.Write([]byte(`{"id":"123","name":"Test Template","data":{"refresh_interval":60,"date_range_from":"now-2h","date_range_to":"now","charts":[],"sections":[]}}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/789/export":
			// Simulate broken export - return 500 error
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Export failed"}`))
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/templates":
			// Return templates with duplicate names for testing
			_, _ = w.Write([]byte(`{"data":[
				{"id":"123","type":"dashboard_template","attributes":{"name":"Test Template","description":"A test template","categories":["test"]}},
				{"id":"234","type":"dashboard_template","attributes":{"name":"Duplicate Template","description":"First duplicate","categories":["test"]}},
				{"id":"345","type":"dashboard_template","attributes":{"name":"Duplicate Template","description":"Second duplicate","categories":["test"]}},
				{"id":"789","type":"dashboard_template","attributes":{"name":"Template with broken export","description":"A template with broken export","categories":["test"]}}
			]}`))
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

				data "logtail_dashboard_template" "by_id" {
					id = "123"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_id", "id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_id", "name", "Test Template"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_id", "data", `{"charts":[],"date_range_from":"now-2h","date_range_to":"now","refresh_interval":60,"sections":[]}`),
				),
				PreConfig: func() {
					t.Log("step 1 - lookup by ID")
				},
			},
			// Step 2 - lookup by ID with matching name
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "by_id_matching" {
					id = "123"
					name = "Test Template"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_id_matching", "id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_id_matching", "name", "Test Template"),
				),
				PreConfig: func() {
					t.Log("step 2 - lookup by ID with matching name")
				},
			},
			// Step 3 - lookup by name
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "by_name" {
					name = "Test Template"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_name", "id", "123"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_name", "name", "Test Template"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_template.by_name", "data", `{"charts":[],"date_range_from":"now-2h","date_range_to":"now","refresh_interval":60,"sections":[]}`),
				),
				PreConfig: func() {
					t.Log("step 3 - lookup by name")
				},
			},
			// Step 4 - lookup by duplicate name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "duplicate" {
					name = "Duplicate Template"
				}
				`,
				ExpectError: regexp.MustCompile(`multiple dashboard templates found with the name "Duplicate Template" - use ID lookup instead, available template IDs: 234, 345`),
				PreConfig: func() {
					t.Log("step 4 - lookup by duplicate name (should error)")
				},
			},
			// Step 4 - lookup by non-existent name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "nonexistent" {
					name = "Non-Existent Template"
				}
				`,
				ExpectError: regexp.MustCompile(`no dashboard template found with name "Non-Existent Template" - available templates: "Duplicate Template", "Template with broken export", "Test Template"`),
				PreConfig: func() {
					t.Log("step 5 - lookup by non-existent name (should error)")
				},
			},
			// Step 5 - lookup by ID with mismatched name (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "mismatch" {
					id = "123"
					name = "Expected Name"
				}
				`,
				ExpectError: regexp.MustCompile(`dashboard template with ID 123 has name "Test Template", but requested name is "Expected Name"`),
				PreConfig: func() {
					t.Log("step 6 - lookup by ID with mismatched name (should error)")
				},
			},
			// Step 7 - lookup by ID with broken export (should error)
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_dashboard_template" "broken_export" {
					id = "789"
				}
				`,
				ExpectError: regexp.MustCompile(`dashboard template "Template with broken export" was found but couldn't be exported`),
				PreConfig: func() {
					t.Log("step 7 - lookup by ID with broken export (should error)")
				},
			},
		},
	})
}
