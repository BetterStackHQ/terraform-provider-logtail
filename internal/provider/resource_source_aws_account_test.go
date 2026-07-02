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

func TestResourceSourceAWSAccount(t *testing.T) {
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
			// The AWS linkage creds must NOT ride along on the source create - they only
			// reach the API via the separate logtail_source_aws_account PATCH.
			if strings.Contains(string(body), "aws_role_arn") || strings.Contains(string(body), "aws_external_id") || strings.Contains(string(body), "aws_account_id") {
				t.Fatalf("source create body should not carry AWS linkage params, got: %s", string(body))
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
			// Step 1 - create the source and connect the account with fresh credentials.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name     = "AWS production"
					platform = "aws"
				}

				resource "logtail_source_aws_account" "aws" {
					source_id       = logtail_source.aws.id
					aws_role_arn    = "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"
					aws_external_id = "ext-123"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_aws_account.aws", "id", "1"),
					resource.TestCheckResourceAttr("logtail_source_aws_account.aws", "source_id", "1"),
					resource.TestCheckResourceAttr("logtail_source_aws_account.aws", "aws_role_arn", "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"),
					patchBodyContains(`"aws_role_arn":"arn:aws:iam::123456789012:role/BetterStackIntegrationRole"`),
					patchBodyContains(`"aws_external_id":"ext-123"`),
				),
			},
			// Step 2 - rotate the credentials in place; the PATCH must carry the new values.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name     = "AWS production"
					platform = "aws"
				}

				resource "logtail_source_aws_account" "aws" {
					source_id       = logtail_source.aws.id
					aws_role_arn    = "arn:aws:iam::123456789012:role/RotatedRole"
					aws_external_id = "ext-456"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					patchBodyContains(`"aws_role_arn":"arn:aws:iam::123456789012:role/RotatedRole"`),
					patchBodyContains(`"aws_external_id":"ext-456"`),
				),
			},
			// Step 3 - drop the linkage resource (keep the source). Delete is state-only:
			// it must not issue a DELETE against the source.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name     = "AWS production"
					platform = "aws"
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

func TestResourceSourceAWSAccountExistingAccount(t *testing.T) {
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
			// Reuse an already-connected account by id instead of pasting back the role ARN.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source" "aws" {
					name     = "AWS production"
					platform = "aws"
				}

				resource "logtail_source_aws_account" "aws" {
					source_id      = logtail_source.aws.id
					aws_account_id = "42"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_aws_account.aws", "aws_account_id", "42"),
					func(s *terraform.State) error {
						body := string(lastPatchBody.Load().([]byte))
						if !strings.Contains(body, `"aws_account_id":"42"`) {
							return fmt.Errorf("PATCH body should contain aws_account_id=42, got: %s", body)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestResourceSourceAWSAccountRequiresCredential(t *testing.T) {
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
			// aws_role_arn without aws_external_id is rejected at plan time.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_aws_account" "aws" {
					source_id    = "1"
					aws_role_arn = "arn:aws:iam::123456789012:role/BetterStackIntegrationRole"
				}
				`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`"aws_role_arn": all of ` + "`aws_external_id,aws_role_arn`" + ` must be specified`),
			},
		},
	})
}
