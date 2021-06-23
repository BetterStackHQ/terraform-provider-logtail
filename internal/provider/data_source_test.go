package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataMonitor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/sources"

		switch {
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(`{"data":[{"id":"1","attributes":{"name":"Test Source","token":"token123","table_name":"abc","platform":"ubuntu"}}],"pagination":{"next":"..."}}`))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=2":
			_, _ = w.Write([]byte(`{"data":[{"id":"2","attributes":{"name":"Other Test Source","token":"token456","table_name":"def","platform":"ubuntu"}}],"pagination":{"next":null}}`))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var table_name = "def"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_source" "this" {
					table_name = "%s"
				}
				`, table_name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.logtail_source.this", "id"),
					resource.TestCheckResourceAttr("data.logtail_source.this", "table_name", table_name),
					resource.TestCheckResourceAttr("data.logtail_source.this", "platform", "ubuntu"),
				),
			},
		},
	})
}

// TODO: test duplicate
