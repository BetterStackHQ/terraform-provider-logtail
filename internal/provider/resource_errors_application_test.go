package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceErrorsApplication(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/applications"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "token", "generated_by_logtail")
			body = inject(t, body, "ingesting_host", "s1234.us-east-9.betterstackdata.com")
			body = inject(t, body, "table_name", "test_errors_application")

			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodPatch && r.RequestURI == prefix+"/"+id:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(data.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patched, err := json.Marshal(patch)
			if err != nil {
				t.Fatal(err)
			}
			patched = inject(t, patched, "token", "generated_by_logtail")
			patched = inject(t, patched, "ingesting_host", "s1234.us-east-9.betterstackdata.com")
			patched = inject(t, patched, "table_name", "test_errors_application")

			data.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, patched)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var name = "Test Errors Application"
	var platform = "ruby_errors"

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
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_errors_application" "this" {
					name     = "%s"
					platform = "%s"
					data_region = "us-east-9"
					errors_retention = 90
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_errors_application.this", "id"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "name", name),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "ingesting_host", "s1234.us-east-9.betterstackdata.com"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "data_region", "us-east-9"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "errors_retention", "90"),
				),
			},
			// Step 2 - update.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_errors_application" "this" {
					name             = "%s"
					platform         = "%s"
					errors_retention = 60
					ingesting_paused = true
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_errors_application.this", "id"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "name", name),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "ingesting_host", "s1234.us-east-9.betterstackdata.com"),
					resource.TestCheckResourceAttr("logtail_errors_application.this", "errors_retention", "60"),
				),
			},
			// Step 3 - make no changes, check plan is empty (omitted attributes are not controlled)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_errors_application" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				PlanOnly: true,
			},
			// Step 4 - destroy.
			{
				ResourceName:      "logtail_errors_application.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
