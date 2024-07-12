package provider

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestResourceSource(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/sources"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "token", "generated_by_logtail")
			body = inject(t, body, "table_name", "test_source")
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodPatch && r.RequestURI == prefix+"/"+id:
			body, err := ioutil.ReadAll(r.Body)
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
			patched = inject(t, patched, "table_name", "test_source")
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

	var name = "Test Source"
	var platform = "ubuntu"

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

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
				),
			},
			// Step 2 - update.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name              = "%s"
					platform          = "%s"
					logs_retention    = 14
					metrics_retention = 60
   					live_tail_pattern = "{level} {message}"
					ingesting_paused  = true
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
				),
			},
			// Step 3 - make no changes, check plan is empty (omitted attributes are not controlled)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
				}
				`, name, platform),
				PlanOnly: true,
			},
			// Step 4 - destroy.
			{
				ResourceName:      "logtail_source.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func inject(t *testing.T, body json.RawMessage, key string, value interface{}) json.RawMessage {
	// Inject source token.
	computed := make(map[string]interface{})
	if err := json.Unmarshal(body, &computed); err != nil {
		t.Fatal(err)
	}
	computed[key] = value
	body, err := json.Marshal(computed)
	if err != nil {
		t.Fatal(err)
	}

	return body
}
