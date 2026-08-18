package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceSourceAWSLogGroup(t *testing.T) {
	var attributes atomic.Value
	var lastPatchBody atomic.Value
	var deletes int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		collectionPath := "/api/v2/sources/1/aws-log-group-subscriptions"
		memberPath := collectionPath + "/7"

		switch {
		case r.Method == http.MethodPost && r.URL.Path == collectionPath:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			attributes.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"7","attributes":%s}}`, body)))
		case r.Method == http.MethodGet && r.URL.Path == memberPath:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"7","attributes":%s}}`, attributes.Load().([]byte))))
		case r.Method == http.MethodPatch && r.URL.Path == memberPath:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastPatchBody.Store(append([]byte{}, body...))
			merged := map[string]interface{}{}
			if err = json.Unmarshal(attributes.Load().([]byte), &merged); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &merged); err != nil {
				t.Fatal(err)
			}
			updated, err := json.Marshal(merged)
			if err != nil {
				t.Fatal(err)
			}
			attributes.Store(updated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"7","attributes":%s}}`, updated)))
		case r.Method == http.MethodDelete && r.URL.Path == memberPath:
			atomic.AddInt32(&deletes, 1)
			w.WriteHeader(http.StatusNoContent)
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
		CheckDestroy: func(_ *terraform.State) error {
			if got := atomic.LoadInt32(&deletes); got != 1 {
				return fmt.Errorf("expected one override DELETE, got %d", got)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_aws_log_group" "api" {
					source_id  = "1"
					region     = "us-east-1"
					name       = "/aws/lambda/api"
					subscribed = true
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "id", "7"),
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "source_id", "1"),
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "region", "us-east-1"),
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "name", "/aws/lambda/api"),
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "subscribed", "true"),
				),
			},
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_aws_log_group" "api" {
					source_id  = "1"
					region     = "us-east-1"
					name       = "/aws/lambda/api"
					subscribed = false
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_aws_log_group.api", "subscribed", "false"),
					func(_ *terraform.State) error {
						body := string(lastPatchBody.Load().([]byte))
						if !strings.Contains(body, `"subscribed":false`) {
							return fmt.Errorf("PATCH body should disable the subscription, got: %s", body)
						}
						return nil
					},
				),
			},
		},
	})
}
