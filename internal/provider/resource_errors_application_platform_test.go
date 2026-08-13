package provider

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// A recent API change makes the API return platform; the provider must treat
// it as an ordinary read attribute so refresh and import mirror the remote value.
func TestResourceErrorsApplicationPlatformFromAPI(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/applications"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "token", "generated_by_logtail")
			body = inject(t, body, "js_tag_token", "js_tag_generated_by_logtail")
			body = inject(t, body, "ingesting_host", "s1234.us-east-9.betterstackdata.com")
			body = inject(t, body, "table_name", "test_errors_application")
			body = inject(t, body, "team_id", 123456)
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	config := `
	provider "logtail" {
		api_token = "foo"
	}

	resource "logtail_errors_application" "this" {
		name     = "Test Errors Application"
		platform = "ruby_errors"
	}
	`

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
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_errors_application.this", "platform", "ruby_errors"),
				),
			},
			// Step 2 - the platform changes out-of-band; refresh must surface the drift
			// (platform is ForceNew, so the plan proposes a replacement).
			{
				PreConfig: func() {
					data.Store(bytes.Replace(data.Load().([]byte), []byte(`"platform":"ruby_errors"`), []byte(`"platform":"javascript_errors"`), 1))
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
