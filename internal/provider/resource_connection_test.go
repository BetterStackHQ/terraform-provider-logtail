package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceConnection(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/connections"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			// Inject computed fields that API returns
			body = inject(t, body, "host", "us-east-9-connect.betterstackdata.com")
			body = inject(t, body, "port", 443)
			body = inject(t, body, "username", "u3PM1B7BEJgqHXBymIpnfCAs3K02XIZaE")
			body = inject(t, body, "password", "XNFT7RaKtjCyZiQIeR782kykeAxOa4U1eLaKxyd7KDN58xlgCwZ0wEkr7YdoBvXh")
			body = inject(t, body, "created_at", "2025-11-26T14:00:00.000Z")
			body = inject(t, body, "updated_at", "2025-11-26T14:00:00.000Z")
			body = inject(t, body, "sample_query", "curl command example")
			body = inject(t, body, "created_by", map[string]interface{}{"id": "123", "email": "test@example.com"})
			body = inject(t, body, "data_sources", []interface{}{})

			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			// For GET, add the complex fields that might be missing
			body := data.Load().([]byte)
			body = inject(t, body, "created_by", map[string]interface{}{"id": "123", "email": "test@example.com"})
			body = inject(t, body, "data_sources", []interface{}{})
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
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
			// Step 1 - create.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_connection" "this" {
					client_type  = "clickhouse"
					team_names   = ["Test Team"]
					data_region  = "us_east"
					ip_allowlist = ["192.168.1.0/24", "10.0.0.1"]
					valid_until  = "2025-12-31T23:59:59Z"
					note         = "Test connection"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_connection.this", "id"),
					resource.TestCheckResourceAttr("logtail_connection.this", "client_type", "clickhouse"),
					resource.TestCheckResourceAttr("logtail_connection.this", "team_names.#", "1"),
					resource.TestCheckResourceAttr("logtail_connection.this", "team_names.0", "Test Team"),
					resource.TestCheckResourceAttr("logtail_connection.this", "data_region", "us_east"), // Should preserve user value, not API normalized value
					resource.TestCheckResourceAttr("logtail_connection.this", "ip_allowlist.#", "2"),
					resource.TestCheckResourceAttr("logtail_connection.this", "ip_allowlist.0", "192.168.1.0/24"),
					resource.TestCheckResourceAttr("logtail_connection.this", "ip_allowlist.1", "10.0.0.1"),
					resource.TestCheckResourceAttr("logtail_connection.this", "valid_until", "2025-12-31T23:59:59Z"), // Should preserve user value
					resource.TestCheckResourceAttr("logtail_connection.this", "note", "Test connection"),
					resource.TestCheckResourceAttr("logtail_connection.this", "host", "us-east-9-connect.betterstackdata.com"),
					resource.TestCheckResourceAttr("logtail_connection.this", "port", "443"),
					resource.TestCheckResourceAttr("logtail_connection.this", "username", "u3PM1B7BEJgqHXBymIpnfCAs3K02XIZaE"),
					resource.TestCheckResourceAttr("logtail_connection.this", "password", "XNFT7RaKtjCyZiQIeR782kykeAxOa4U1eLaKxyd7KDN58xlgCwZ0wEkr7YdoBvXh"),
					resource.TestCheckResourceAttr("logtail_connection.this", "created_at", "2025-11-26T14:00:00.000Z"),
					resource.TestCheckResourceAttr("logtail_connection.this", "sample_query", "curl command example"),
				),
			},
		},
	})
}
