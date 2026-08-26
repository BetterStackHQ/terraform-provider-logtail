package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceExplorationAlertVariableValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		if r.Method != http.MethodGet || r.RequestURI != "/api/v2/explorations/1/alerts" {
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"10","attributes":{"name":"High Error Rate","alert_type":"threshold","operator":"higher_than","value":100,"variable_values":[{"name":"level","values":["error"],"selected_label":"Error"}],"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}}]}`))
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{{
			Config: `
			provider "logtail" {
				api_token = "foo"
			}

			data "logtail_exploration_alert" "this" {
				exploration_id = "1"
				name           = "High Error Rate"
			}
			`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.logtail_exploration_alert.this", "id", "1/10"),
				resource.TestCheckResourceAttr("data.logtail_exploration_alert.this", "variable_values.#", "1"),
				resource.TestCheckResourceAttr("data.logtail_exploration_alert.this", "variable_values.0.name", "level"),
				resource.TestCheckResourceAttr("data.logtail_exploration_alert.this", "variable_values.0.values.0", "error"),
				resource.TestCheckResourceAttr("data.logtail_exploration_alert.this", "variable_values.0.selected_label", "Error"),
			),
		}},
	})
}
