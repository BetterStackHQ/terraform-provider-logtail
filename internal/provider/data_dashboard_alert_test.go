package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceDashboardAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v2/dashboards/1/charts/10/alerts":
			_, _ = w.Write([]byte(`{"data":[{"id":"100","attributes":{"name":"High Error Rate","alert_type":"threshold","operator":"higher_than","value":100,"query_period":300,"confirmation_period":60,"recovery_period":0,"check_period":300,"series_names":[],"source_mode":"source_variable","source_platforms":[],"paused":false,"call":false,"sms":false,"email":true,"push":true,"critical_alert":false,"incident_per_series":false,"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}}]}`))
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

				data "logtail_dashboard_alert" "this" {
					dashboard_id = "1"
					chart_id     = "10"
					name         = "High Error Rate"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "id", "1/10/100"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "name", "High Error Rate"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "alert_type", "threshold"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "operator", "higher_than"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "value", "100"),
					resource.TestCheckResourceAttr("data.logtail_dashboard_alert.this", "email", "true"),
				),
			},
		},
	})
}
