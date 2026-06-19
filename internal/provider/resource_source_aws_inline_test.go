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

// Connecting the AWS account inline on the logtail_source resource: the credentials ride along
// on the source create/update and the server runs the connect manager. They're write-only — the
// API strips them from responses — so they must not produce a perpetual diff.
func TestResourceSourceAWSInline(t *testing.T) {
	var data atomic.Value
	var lastCreateBody atomic.Value
	var lastPatchBody atomic.Value

	// strip mimics the real Sources API: the AWS linkage params are write-only and never echoed
	// back as source attributes.
	strip := func(t *testing.T, body []byte) []byte {
		m := make(map[string]interface{})
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatal(err)
		}
		delete(m, "aws_account_id")
		delete(m, "aws_role_arn")
		delete(m, "aws_external_id")
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

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
			lastCreateBody.Store(append([]byte{}, body...))
			body = strip(t, body)
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
			if err = json.Unmarshal(strip(t, body), &patch); err != nil {
				t.Fatal(err)
			}
			patched, err := json.Marshal(patch)
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

	createBodyContains := func(needle string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			body := string(lastCreateBody.Load().([]byte))
			if !strings.Contains(body, needle) {
				return fmt.Errorf("create body should contain %s, got: %s", needle, body)
			}
			return nil
		}
	}
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
			// Step 1 - connect the account at create time via inline fields on the source.
			// A clean post-apply plan proves the write-only creds don't drift.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name            = "AWS production"
					platform        = "aws"
					aws_role_arn    = "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"
					aws_external_id = "ext-123"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source.aws", "id", "1"),
					resource.TestCheckResourceAttr("logtail_source.aws", "aws_role_arn", "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"),
					createBodyContains(`"aws_role_arn":"arn:aws:iam::123456789012:role/BetterStackIntegrationRole"`),
					createBodyContains(`"aws_external_id":"ext-123"`),
				),
			},
			// Step 2 - rotate the credentials in place; the PATCH must re-send the full set.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name            = "AWS production"
					platform        = "aws"
					aws_role_arn    = "arn:aws:iam::123456789012:role/RotatedRole"
					aws_external_id = "ext-456"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					patchBodyContains(`"aws_role_arn":"arn:aws:iam::123456789012:role/RotatedRole"`),
					patchBodyContains(`"aws_external_id":"ext-456"`),
				),
			},
		},
	})
}

// aws_role_arn without aws_external_id is rejected at plan time (RequiredWith pairing).
func TestResourceSourceAWSInlineRequiresExternalID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Unexpected request " + r.Method + " " + r.RequestURI + " — config should fail validation before any API call")
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name         = "AWS production"
					platform     = "aws"
					aws_role_arn = "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"
				}
				`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`"aws_role_arn": all of ` + "`aws_external_id,aws_role_arn`" + ` must be specified`),
			},
		},
	})
}

// The AWS linkage params are rejected at plan time on a non-aws source.
func TestResourceSourceAWSInlineRejectsNonAWSPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Unexpected request " + r.Method + " " + r.RequestURI + " — config should fail validation before any API call")
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "docker" {
					name            = "Docker"
					platform        = "docker"
					aws_role_arn    = "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"
					aws_external_id = "ext-123"
				}
				`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`aws_role_arn can only be set when platform is "aws"`),
			},
		},
	})
}
