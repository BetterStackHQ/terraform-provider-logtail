package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestResourceSource(t *testing.T) {
	var data atomic.Value
	// Track the last request body for assertions
	var lastRequestBody atomic.Value
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
			// Store raw request body for assertions
			lastRequestBody.Store(append([]byte{}, body...))
			body = inject(t, body, "token", "generated_by_logtail")
			body = inject(t, body, "ingesting_host", "in.logs.betterstack.com")
			body = inject(t, body, "table_name", "test_source")
			body = inject(t, body, "team_id", 123456)

			// Handle custom_bucket - remove secret_access_key from response as API doesn't return it
			body = simulateCustomBucketAPI(t, body)

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
			// Store raw request body for assertions
			lastRequestBody.Store(append([]byte{}, body...))
			// The real API rejects custom_bucket on update - the provider must never send it.
			if rejectCustomBucketUpdate(w, body) {
				return
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
			patched = inject(t, patched, "team_id", 123456)

			// Handle custom_bucket - remove secret_access_key from response as API doesn't return it
			patched = simulateCustomBucketAPI(t, patched)

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
					source_group_id = 123
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "team_id", "123456"),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_host", "in.logs.betterstack.com"),
					resource.TestCheckResourceAttr("logtail_source.this", "data_region", "eu-hel-1-legacy"),
					resource.TestCheckResourceAttr("logtail_source.this", "source_group_id", "123"),
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
					source_group_id = 456
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "team_id", "123456"),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_host", "in.logs.betterstack.com"),
					resource.TestCheckResourceAttr("logtail_source.this", "data_region", "eu-hel-1-legacy"),
					resource.TestCheckResourceAttr("logtail_source.this", "source_group_id", "456"),
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
					source_group_id = 456
				}
				`, name, platform),
				PlanOnly: true,
			},
			// Step 4 - remove source_group_id from config (null), verify no plan change
			// This tests that null means "don't care" and doesn't cause drift
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				PlanOnly: true,
			},
			// Step 5 - set source_group_id = 0 (remove from group), verify 0 is sent to API
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name            = "%s"
					platform        = "%s"
					source_group_id = 0
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "source_group_id", "0"),
					// Verify source_group_id=0 was sent to API (to remove from group)
					func(s *terraform.State) error {
						body := string(lastRequestBody.Load().([]byte))
						if !strings.Contains(body, `"source_group_id":0`) {
							return fmt.Errorf("request body should contain source_group_id=0, got: %s", body)
						}
						return nil
					},
				),
			},
			// Step 6 - import
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
					skip_ssl_verify       = true
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "team_id", "123456"),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform_scrape),
					resource.TestCheckResourceAttr("logtail_source.this", "token", "generated_by_logtail"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_urls.#", "2"),
					resource.TestCheckResourceAttr("logtail_source.this", "scrape_frequency_secs", "60"),
					resource.TestCheckResourceAttr("logtail_source.this", "skip_ssl_verify", "true"),
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
					skip_ssl_verify = false
				}
				`, name, platform_scrape),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "team_id", "123456"),
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
					resource.TestCheckResourceAttr("logtail_source.this", "skip_ssl_verify", "false"),
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

	// Test custom_bucket functionality
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket. name is omitted: the API derives it from the
			// endpoint URL, and state must pick up the derived value.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "1"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.name", "my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.endpoint", "https://s3.us-east-1.amazonaws.com/my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.access_key_id", "AKIAIOSFODNN7EXAMPLE"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.secret_access_key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.keep_data_after_retention", "false"),
					// an omitted name is not sent to the API
					func(s *terraform.State) error {
						body := string(lastRequestBody.Load().([]byte))
						if strings.Contains(body, `"name":"my-test-bucket"`) {
							return fmt.Errorf("omitted custom_bucket.name should not be sent to the API, got: %s", body)
						}
						return nil
					},
				),
			},
			// Step 2 - update an unrelated field; custom_bucket must not be sent (the mock
			// returns 422 like the real API if it is) and must stay intact in state.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
					custom_bucket {
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "ingesting_paused", "true"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "1"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.name", "my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.endpoint", "https://s3.us-east-1.amazonaws.com/my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.access_key_id", "AKIAIOSFODNN7EXAMPLE"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.secret_access_key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.keep_data_after_retention", "false"),
				),
			},
			// Step 3 - setting name to the stored (derived) value is fine - empty plan.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
					custom_bucket {
						name              = "my-test-bucket"
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly: true,
			},
			// Step 4 - changing name is a benign state-only update: the API ignores the value,
			// so nothing is sent (the mock 422s any custom_bucket PATCH) and state keeps the
			// configured name from now on.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
					custom_bucket {
						name              = "renamed-bucket"
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.name", "renamed-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.endpoint", "https://s3.us-east-1.amazonaws.com/my-test-bucket"),
				),
			},
			// Step 5 - changing the endpoint fails at plan time.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
					custom_bucket {
						endpoint          = "https://s3.us-east-1.amazonaws.com/other-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket\.endpoint cannot be changed once set`),
			},
			// Step 6 - changing the secret fails at plan time (it is only fillable after import).
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
					custom_bucket {
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "differentsecret"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket\.secret_access_key cannot be changed once set`),
			},
		},
	})

	// Test that custom_bucket cannot be added to an existing source
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create without custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				Check: resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "0"),
			},
			// Step 2 - adding custom_bucket to the existing source fails at plan time
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket can only be set when creating the resource`),
			},
		},
	})

	// Test custom_bucket keep data after retention functionality
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket and keep data after retention
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						endpoint                  = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id             = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key         = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
						keep_data_after_retention = true
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "team_id", "123456"),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "1"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.name", "my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.endpoint", "https://s3.us-east-1.amazonaws.com/my-test-bucket"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.access_key_id", "AKIAIOSFODNN7EXAMPLE"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.secret_access_key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.keep_data_after_retention", "true"),
				),
			},
		},
	})

	// Test custom_bucket validation errors
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 0 - empty credentials fail validation. Missing CI secrets surface as empty
			// strings (not null), and the API silently creates the source without any bucket
			// when all bucket fields are blank - this must fail loudly at plan time instead.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						endpoint          = ""
						access_key_id     = ""
						secret_access_key = ""
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`expected "custom_bucket\.0\.endpoint" to not be an empty string`),
			},
			// Step 1 - custom_bucket missing endpoint (schema validation)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`The argument "endpoint" is required`),
			},
			// Step 3 - custom_bucket missing access_key_id (schema validation)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-test-bucket"
						endpoint          = "https://s3.amazonaws.com"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`The argument "access_key_id" is required`),
			},
			// Step 4 - custom_bucket missing secret_access_key (schema validation)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name          = "my-test-bucket"
						endpoint      = "https://s3.amazonaws.com"
						access_key_id = "AKIAIOSFODNN7EXAMPLE"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`The argument "secret_access_key" is required`),
			},
		},
	})

	// Test custom_bucket removal validation
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-own-label"
						endpoint          = "https://s3.us-east-1.amazonaws.com/my-test-bucket"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "1"),
					// a configured name is kept in state even though the API stores the
					// endpoint-derived one
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.0.name", "my-own-label"),
				),
			},
			// Step 2 - try to remove custom_bucket (should fail)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket cannot be removed once set - it is a create-only field`),
			},
		},
	})

	// Test source without custom_bucket (should work as before)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create without custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_source.this", "custom_bucket.#", "0"),
				),
			},
		},
	})

	// Test VRL transformation functionality
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with VRL transformation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					vrl_transformation = <<EOT
					# Expected msg format: [svc:router] GET /api/health succeeded in 12.345ms
					.duration_ms = extract(.message, "in (\d+(?:\.\d+)?)ms")
					.service_name = extract(.message, "\[svc:([a-zA-Z_-])\]")
					EOT
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "id"),
					resource.TestCheckResourceAttr("logtail_source.this", "name", name),
					resource.TestCheckResourceAttr("logtail_source.this", "platform", platform),
					resource.TestCheckResourceAttrSet("logtail_source.this", "vrl_transformation"),
				),
			},
			// Step 2 - update VRL transformation with different formatting (should not cause diff due to normalization)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					vrl_transformation = <<EOT
					# Expected msg format: [svc:router] GET /api/health succeeded in 12.345ms
					
					.duration_ms = extract(.message, "in (\d+(?:\.\d+)?)ms") .
					.service_name = extract(.message, "\[svc:([a-zA-Z_-])\]") .
					.
					EOT
				}
				`, name, platform),
				PlanOnly: true, // Should not show any changes due to DiffSuppressFunc
			},
			// Step 3 - update VRL transformation with actual changes
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					vrl_transformation = <<EOT
					# Updated VRL transformation
					.duration_ms = extract(.message, "in (\d+(?:\.\d+)?)ms")
					.service_name = extract(.message, "\[svc:([a-zA-Z_-]+)\]")
					.method = extract(.message, "([A-Z]+) /")
					EOT
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "vrl_transformation"),
				),
			},
			// Step 4 - remove VRL transformation (set to empty)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
					vrl_transformation = ""
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "vrl_transformation", ""),
				),
			},
		},
	})
}

func TestResourceSourcePerTypeVrl(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			body = inject(t, body, "team_id", 123456)
			// Mirror the real serializer: vrl_transformation is the deprecated alias for
			// vrl_transformation_logs, so the API returns both populated with the logs value.
			var attrs map[string]interface{}
			if err := json.Unmarshal(body, &attrs); err != nil {
				t.Fatal(err)
			}
			if logs, ok := attrs["vrl_transformation_logs"]; ok {
				body = inject(t, body, "vrl_transformation", logs)
			} else if alias, ok := attrs["vrl_transformation"]; ok {
				body = inject(t, body, "vrl_transformation_logs", alias)
			}
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
			merged := make(map[string]interface{})
			if err := json.Unmarshal(data.Load().([]byte), &merged); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &merged); err != nil {
				t.Fatal(err)
			}
			if logs, ok := merged["vrl_transformation_logs"]; ok {
				merged["vrl_transformation"] = logs
			}
			patched, err := json.Marshal(merged)
			if err != nil {
				t.Fatal(err)
			}
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

	name := "Test Source"
	platform := "ubuntu"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// vrl_transformation and vrl_transformation_logs are mutually exclusive (deprecated alias).
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name                    = "%s"
					platform                = "%s"
					vrl_transformation      = ".a = 1\n."
					vrl_transformation_logs = ".b = 2\n."
				}
				`, name, platform),
				ExpectError: regexp.MustCompile(`(?s)conflicts with`),
			},
			// Per-type logs + spans VRL round-trips independently.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name                     = "%s"
					platform                 = "%s"
					vrl_transformation_logs  = ".message = upcase!(.message)\n."
					vrl_transformation_spans = ".name = downcase!(.name)\n."
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_source.this", "vrl_transformation_logs"),
					resource.TestCheckResourceAttrSet("logtail_source.this", "vrl_transformation_spans"),
				),
			},
			// Clearing a per-type VRL with "" takes effect (the config-aware suppressor lets the
			// empty value through; Computed would have kept the old value).
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name                     = "%s"
					platform                 = "%s"
					vrl_transformation_logs  = ""
					vrl_transformation_spans = ".name = downcase!(.name)\n."
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "vrl_transformation_logs", ""),
				),
			},
		},
	})
}

func TestResourceSourceBlockedMetrics(t *testing.T) {
	var data atomic.Value
	// The real serializer always returns blocked_metrics (an empty list when unset).
	ensureBlockedMetrics := func(body json.RawMessage) json.RawMessage {
		var attrs map[string]interface{}
		if err := json.Unmarshal(body, &attrs); err != nil {
			t.Fatal(err)
		}
		if _, ok := attrs["blocked_metrics"]; !ok {
			body = inject(t, body, "blocked_metrics", []string{})
		}
		return body
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			body = inject(t, body, "team_id", 123456)
			body = ensureBlockedMetrics(body)
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
			merged := make(map[string]interface{})
			if err := json.Unmarshal(data.Load().([]byte), &merged); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &merged); err != nil {
				t.Fatal(err)
			}
			patched, err := json.Marshal(merged)
			if err != nil {
				t.Fatal(err)
			}
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

	name := "Test Source"
	platform := "ubuntu"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// No blocked_metrics in config; the API echoes an empty list, which must not drift.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "blocked_metrics.#", "0"),
				),
			},
			// Set blocked metrics.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name            = "%s"
					platform        = "%s"
					blocked_metrics = ["node_cpu_seconds_total", "process_open_fds"]
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "blocked_metrics.#", "2"),
					resource.TestCheckResourceAttr("logtail_source.this", "blocked_metrics.0", "node_cpu_seconds_total"),
				),
			},
			// Clear with an empty list.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name            = "%s"
					platform        = "%s"
					blocked_metrics = []
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "blocked_metrics.#", "0"),
				),
			},
		},
	})
}

func TestResourceSourceCodeMapping(t *testing.T) {
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
			body = inject(t, body, "team_id", 123456)
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
			patched = inject(t, patched, "team_id", 123456)
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

	var name = "Test Source Code Mapping"
	var platform = "ubuntu"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with code mapping fields set.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name                     = "%s"
					platform                 = "%s"
					code_mapping_stack_root  = "/usr/src/app/"
					code_mapping_source_root = "apps/backend/"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "code_mapping_stack_root", "/usr/src/app/"),
					resource.TestCheckResourceAttr("logtail_source.this", "code_mapping_source_root", "apps/backend/"),
				),
			},
			// Step 2 - update code mapping fields.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name                     = "%s"
					platform                 = "%s"
					code_mapping_stack_root  = "/opt/app/"
					code_mapping_source_root = "src/"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "code_mapping_stack_root", "/opt/app/"),
					resource.TestCheckResourceAttr("logtail_source.this", "code_mapping_source_root", "src/"),
				),
			},
		},
	})
}

// TestResourceSourceImportDataRegion covers importing with data_region omitted: the API returns
// the cluster name ("eu-nbg-2"), not the region identifier given at creation, so the plan stays empty.
func TestResourceSourceImportDataRegion(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			body = inject(t, body, "team_id", 123456)
			// API returns the cluster name, not the region identifier given at creation.
			body = inject(t, body, "data_region", "eu-nbg-2")
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

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Create with data_region omitted; the cluster name lands in state.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "Imported Source"
					platform = "ubuntu"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.this", "data_region", "eu-nbg-2"),
				),
			},
			// Import: state matches, no spurious data_region diff.
			{
				ResourceName:      "logtail_source.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Re-plan with data_region omitted: empty plan.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "this" {
					name     = "Imported Source"
					platform = "ubuntu"
				}
				`,
				PlanOnly: true,
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

// simulateCustomBucketAPI mimics the server's custom_bucket handling: the stored bucket name is
// always the one parsed out of the endpoint URL (any name sent by the caller is ignored), and
// secret_access_key is write-only, never returned.
func simulateCustomBucketAPI(t *testing.T, body json.RawMessage) json.RawMessage {
	response := make(map[string]interface{})
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}

	if customBucket, ok := response["custom_bucket"].(map[string]interface{}); ok {
		delete(customBucket, "secret_access_key")
		endpoint, _ := customBucket["endpoint"].(string)
		if i := strings.LastIndex(endpoint, "/"); i > len("https:/") {
			customBucket["name"] = endpoint[i+1:]
		}
		response["custom_bucket"] = customBucket
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	return body
}

// rejectCustomBucketUpdate mimics the server's PATCH guard: custom_bucket cannot be updated after
// creation. Returns true if it wrote the 422 response.
func rejectCustomBucketUpdate(w http.ResponseWriter, body []byte) bool {
	patch := make(map[string]interface{})
	if err := json.Unmarshal(body, &patch); err != nil {
		return false
	}
	if _, ok := patch["custom_bucket"]; !ok {
		return false
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	_, _ = w.Write([]byte(`{"errors":"Custom S3 storage cannot be updated after creation","invalid_attributes":["custom_bucket"]}`))
	return true
}
