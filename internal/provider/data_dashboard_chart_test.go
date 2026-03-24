package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceDashboardChart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/1/charts":
			_, _ = w.Write([]byte(`{"data":[{"id":"10","attributes":{"chart_type":"line_chart","name":"Request Rate","description":"","x":0,"y":0,"w":6,"h":4,"settings":{},"queries":[{"id":1,"query_type":"sql_expression","name":"","sql_query":"SELECT count(*) FROM logs","source_variable":"source","where_condition":"","static_text":""}],"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}}]}`))
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

				data "logtail_dashboard_chart" "this" {
					dashboard_id = "1"
					name         = "Request Rate"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_chart.this", "id", "1/10"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_chart.this", "chart_type", "line_chart"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_chart.this", "name", "Request Rate"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_chart.this", "w", "6"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_chart.this", "h", "4"),
					resource.TestCheckResourceAttrSet("data.logtail_dashboard_chart.this", "created_at"),
				),
			},
		},
	})
}
