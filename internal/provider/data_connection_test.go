package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestDataSourceConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.RequestURI == "/api/v1/connections/1":
			// The API returns `password`, but the data source schema has no such
			// attribute - the shared connectionCopyAttrs must skip it, not panic.
			_, _ = w.Write([]byte(`{"data":{"id":"1","attributes":{
				"client_type":"clickhouse",
				"team_names":["Test Team"],
				"data_region":"us_east",
				"host":"us-east-9-connect.betterstackdata.com",
				"port":443,
				"username":"u3PM1B7BEJgqHXBymIpnfCAs3K02XIZaE",
				"password":"XNFT7RaKtjCyZiQIeR782kykeAxOa4U1eLaKxyd7KDN58xlgCwZ0wEkr7YdoBvXh",
				"created_at":"2025-11-26T14:00:00.000Z",
				"sample_query":"curl command example",
				"data_sources":[]
			}}}`))
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
			// Step 1 - lookup by ID.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_connection" "this" {
					id = "1"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_connection.this", "id", "1"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "client_type", "clickhouse"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "team_names.#", "1"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "team_names.0", "Test Team"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "data_region", "us_east"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "host", "us-east-9-connect.betterstackdata.com"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "port", "443"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "username", "u3PM1B7BEJgqHXBymIpnfCAs3K02XIZaE"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "created_at", "2025-11-26T14:00:00.000Z"),
					resource.TestCheckResourceAttr("data.logtail_connection.this", "sample_query", "curl command example"),
					resource.TestCheckNoResourceAttr("data.logtail_connection.this", "password"),
				),
				PreConfig: func() {
					t.Log("step 1 - lookup by ID")
				},
			},
		},
	})
}
