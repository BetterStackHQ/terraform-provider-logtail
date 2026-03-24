package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceDashboardSection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/1/sections":
			_, _ = w.Write([]byte(`{"data":[{"id":"10","attributes":{"name":"Performance","y":8,"collapsed":false,"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}}]}`))
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

				data "logtail_dashboard_section" "this" {
					dashboard_id = "1"
					name         = "Performance"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_section.this", "id", "1/10"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_section.this", "name", "Performance"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_section.this", "y", "8"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_section.this", "collapsed", "false"),
				),
			},
		},
	})
}
