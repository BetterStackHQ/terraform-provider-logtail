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

func errorsReleaseTestServer(t *testing.T, canonicalApplicationID int) *httptest.Server {
	var data atomic.Value
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/releases"
		id := "456"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var in struct {
				Version      *string  `json:"version"`
				Applications []int    `json:"applications"`
				Environment  []string `json:"environment"`
			}
			if err := json.Unmarshal(body, &in); err != nil {
				t.Fatal(err)
			}
			fields := make(map[string]json.RawMessage)
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			for key := range fields {
				if key != "version" && key != "applications" && key != "environment" {
					t.Fatal("Unexpected key in create request: " + key)
				}
			}
			if in.Version == nil || *in.Version == "" {
				t.Fatal("Missing version in create request: " + string(body))
			}
			if len(in.Applications) != 1 {
				t.Fatal("Expected exactly one application in create request: " + string(body))
			}
			// The server merges environments the release is seen in over time; store an
			// extra one right away so reads returning merged environments are exercised.
			environments, err := json.Marshal(append(in.Environment, "staging"))
			if err != nil {
				t.Fatal(err)
			}
			data.Store([]byte(fmt.Sprintf(
				`{"version":%q,"application_id":%d,"environments":%s,"first_seen_at":"2026-08-18T09:00:00.000Z","last_seen_at":null,"origin":"reported"}`,
				*in.Version, canonicalApplicationID, environments)))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"version":%q,"applications":[%d],"environment":%s,"releases":[{"id":%s,"application_id":%d}]}`,
				*in.Version, in.Applications[0], environments, id, canonicalApplicationID)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			stored, _ := data.Load().([]byte)
			if stored == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"type":"release","attributes":%s}}`, id, stored)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
}

func TestResourceErrorsRelease(t *testing.T) {
	server := errorsReleaseTestServer(t, 123)
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

				resource "logtail_errors_release" "this" {
					application_id = 123
					version        = "v1.2.3"
					environments   = ["production"]
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_errors_release.this", "id", "456"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "application_id", "123"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "version", "v1.2.3"),
					// The server merges more environments over time; state keeps the configured value.
					resource.TestCheckResourceAttr("logtail_errors_release.this", "environments.#", "1"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "environments.0", "production"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "first_seen_at", "2026-08-18T09:00:00.000Z"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "origin", "reported"),
				),
			},
			// Step 2 - change version (should force recreation, there is no update endpoint).
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_errors_release" "this" {
					application_id = 123
					version        = "v2.0.0"
					environments   = ["production"]
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_errors_release.this", "id", "456"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "version", "v2.0.0"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "origin", "reported"),
				),
			},
			// Step 3 - import.
			{
				ResourceName:      "logtail_errors_release.this",
				ImportState:       true,
				ImportStateVerify: true,
				// environments is never read back from the API (the server merges values over time).
				ImportStateVerifyIgnore: []string{"environments"},
			},
		},
	})
}

func TestResourceErrorsReleaseCanonicalApplication(t *testing.T) {
	// The API resolves the sent application to its canonical sibling; the response value
	// is what ends up in state.
	server := errorsReleaseTestServer(t, 124)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_errors_release" "this" {
					application_id = 123
					version        = "v1.2.3"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_errors_release.this", "id", "456"),
					resource.TestCheckResourceAttr("logtail_errors_release.this", "application_id", "124"),
				),
				// The canonical application ID differs from the configured one, so the
				// follow-up plan proposes replacement until the config is corrected.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
