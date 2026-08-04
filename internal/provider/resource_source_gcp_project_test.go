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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceSourceGCPProject(t *testing.T) {
	var data atomic.Value
	var lastPatchBody atomic.Value
	var sourceDeletes int32

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
			// The GCP linkage creds must NOT ride along on the source create - they only
			// reach the API via the separate logtail_source_gcp_project PATCH.
			if strings.Contains(string(body), "gcp_project_id") || strings.Contains(string(body), "gcp_project_number") || strings.Contains(string(body), "gcp_account_id") {
				t.Fatalf("source create body should not carry GCP linkage params, got: %s", string(body))
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
			lastPatchBody.Store(append([]byte{}, body...))
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
			data.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, patched)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			atomic.AddInt32(&sourceDeletes, 1)
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	patchBodyContains := func(needle string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			body := string(lastPatchBody.Load().([]byte))
			if !strings.Contains(body, needle) {
				return fmt.Errorf("PATCH body should contain %s, got: %s", needle, body)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create the source and connect the project with fresh credentials.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "gcp" {
					name     = "GCP production"
					platform = "gcp"
				}

				resource "logtail_source_gcp_project" "gcp" {
					source_id          = logtail_source.gcp.id
					gcp_project_id     = "my-gcp-project"
					gcp_project_number = "587153488274"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_gcp_project.gcp", "id", "1"),
					resource.TestCheckResourceAttr("logtail_source_gcp_project.gcp", "source_id", "1"),
					resource.TestCheckResourceAttr("logtail_source_gcp_project.gcp", "gcp_project_id", "my-gcp-project"),
					patchBodyContains(`"gcp_project_id":"my-gcp-project"`),
					patchBodyContains(`"gcp_project_number":"587153488274"`),
				),
			},
			// Step 2 - reconnect a different project in place; the PATCH must carry the new values.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "gcp" {
					name     = "GCP production"
					platform = "gcp"
				}

				resource "logtail_source_gcp_project" "gcp" {
					source_id          = logtail_source.gcp.id
					gcp_project_id     = "my-other-project"
					gcp_project_number = "912345678901"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					patchBodyContains(`"gcp_project_id":"my-other-project"`),
					patchBodyContains(`"gcp_project_number":"912345678901"`),
				),
			},
			// Step 3 - drop the linkage resource (keep the source). Delete is state-only:
			// it must not issue a DELETE against the source.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "gcp" {
					name     = "GCP production"
					platform = "gcp"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						if n := atomic.LoadInt32(&sourceDeletes); n != 0 {
							return fmt.Errorf("removing the linkage must not DELETE the source, saw %d DELETE(s)", n)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestResourceSourceGCPProjectExistingAccount(t *testing.T) {
	var data atomic.Value
	var lastPatchBody atomic.Value
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
			lastPatchBody.Store(append([]byte{}, body...))
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
			// Reuse an already-connected project by id instead of pasting back the credentials.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "gcp" {
					name     = "GCP production"
					platform = "gcp"
				}

				resource "logtail_source_gcp_project" "gcp" {
					source_id      = logtail_source.gcp.id
					gcp_account_id = "42"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_gcp_project.gcp", "gcp_account_id", "42"),
					func(s *terraform.State) error {
						body := string(lastPatchBody.Load().([]byte))
						if !strings.Contains(body, `"gcp_account_id":"42"`) {
							return fmt.Errorf("PATCH body should contain gcp_account_id=42, got: %s", body)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestResourceSourceGCPProjectRequiresCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Unexpected request " + r.Method + " " + r.RequestURI + " - config should fail validation before any API call")
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
			// gcp_project_id without gcp_project_number is rejected at plan time.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_gcp_project" "gcp" {
					source_id      = "1"
					gcp_project_id = "my-gcp-project"
				}
				`,
				PlanOnly: true,
				// The trailing "must be specified" is left off the pattern: terraform wraps
				// this longer message across lines, which a single-line regex can't span.
				ExpectError: regexp.MustCompile(`"gcp_project_id": all of ` + "`gcp_project_id,gcp_project_number`"),
			},
		},
	})
}
