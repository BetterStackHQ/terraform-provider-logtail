package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
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
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "token", "generated_by_logtail")
			body = inject(t, body, "ingesting_host", "in.logs.betterstack.com")
			body = inject(t, body, "table_name", "test_source")
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
			patched = inject(t, patched, "ingesting_host", "in.logs.betterstack.com")
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
					data_region      = "eu-hel-1-legacy"
					resource_group_id = 123
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_host", "in.logs.betterstack.com"),
					resource.TestCheckResourceAttr("logtail_source.this", "data_region", "eu-hel-1-legacy"),
					resource.TestCheckResourceAttr("logtail_source.this", "resource_group_id", "123"),
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
					resource_group_id = 456
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_host", "in.logs.betterstack.com"),
					resource.TestCheckResourceAttr("logtail_source.this", "data_region", "eu-hel-1-legacy"),
					resource.TestCheckResourceAttr("logtail_source.this", "resource_group_id", "456"),
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
					resource_group_id = 456
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

	var platform_scrape = "prometheus_scrape"

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
					scrape_urls      = ["http://localhost:9100/metrics", "http://localhost:9101/metrics"]
					scrape_frequency_secs = 60
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform_scrape),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_urls.#", "2"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_frequency_secs", "60"),
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
					scrape_urls      = ["http://localhost:9100/metrics"]
					scrape_frequency_secs = 30
					scrape_request_basic_auth_user = "user1"
					scrape_request_basic_auth_password = "password1"
					scrape_request_headers = [
						{ 
							name = "Authorization",
							value = "Bearer foo"
						}
					]
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform_scrape),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_urls.#", "1"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_urls.0", "http://localhost:9100/metrics"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_frequency_secs", "30"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.#", "1"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.name", "Authorization"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.value", "Bearer foo"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_basic_auth_user", "user1"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_basic_auth_password", "password1"),
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
					scrape_urls      = ["http://localhost:9100/metrics"]
					scrape_frequency_secs = 30
					scrape_request_basic_auth_user = "user1"
					scrape_request_basic_auth_password = "password1"
					scrape_request_headers = [
						{ 
							name = "Authorization",
							value = "Bearer foo"
						}
					]
				}
				`, name, platform_scrape),
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
					scrape_request_headers = [
						{ 
							name = "X-TEST",
							value = "test"
						}
					]
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.name", "X-TEST"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.value", "test"),
				),
			},
			// Step 2 - add another request header.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							name = "X-TEST",
							value = "test"
						},
						{ 
							name = "X-TEST-2",
							value = "test-2"	
						}
					]
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.name", "X-TEST"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.value", "test"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.1.name", "X-TEST-2"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.1.value", "test-2"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.#", "2"),
				),
			},
			// Step 3 - remove the first header.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							name = "X-TEST-2",
							value = "test-2"	
						}
					]
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.name", "X-TEST-2"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_request_headers.0.value", "test-2"),
					resource.TestCheckNoResourceAttr("logtail_source.this", "scrape_request_headers.1.name"),
				),
			},
			// Step 4 - invalid header with empty name.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							name = "",
							value = "test"	
						}
					]
				}
				`, name, platform_scrape),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid request header map\[name: value:test\]: must contain 'name' key with a non-empty string value`),
			},
			// Step 5 - invalid header with empty value.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							name = "X-TEST",
							value = ""	
						}
					]
				}
				`, name, platform_scrape),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid request header map\[name:X-TEST value:\]: must contain 'value' key with a non-empty string value`),
			},
			// Step 6 - invalid header with extra keys.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{
							name  = "X-TEST"
							value = "test"
							extra = "invalid"
						}
					]
				}
				`, name, platform_scrape),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid request header map\[extra:invalid name:X-TEST value:test\]: must only contain 'name' and 'value' keys`),
			},
			// Step 7 - invalid header with incorrect format.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							"X-TEST" = "test"
						}
					]
				}
				`, name, platform_scrape),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid request header map\[X-TEST:test\]: must contain 'name' key with a non-empty string value`),
			},
			// Step 8 - change of data region
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					scrape_request_headers = [
						{ 
							name = "X-TEST"
							value = "test"
						}
					]
					data_region = "new_data_region"
				}
				`, name, platform_scrape),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`data_region cannot be changed after source is created`),
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
