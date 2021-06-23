package provider

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestProvider(t *testing.T) {
	if err := New().InternalValidate(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderInit(t *testing.T) {
	var success int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		exptectedUA := "terraform-provider-logtail/0.0.0-0"
		if r.Header.Get("User-Agent") != exptectedUA {
			t.Fatalf("User-Agent: got %q, want %q", r.Header.Get("User-Agent"), exptectedUA)
		}

		atomic.StoreInt32(&success, 1)
		_, _ = w.Write([]byte(`{"data":[{"id":"1","attributes":{"name":"Test source","platform":"ubuntu","token":"token123","table_name":"abc","ingesting_paused":false}}],"pagination":{"next":null}}`))
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL), WithVersion("0.0.0-0")), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create.
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				data "logtail_source" "this" {
					table_name = "abc"
				}
				`,
			},
		},
	})

	if atomic.LoadInt32(&success) != int32(1) {
		t.Fatalf("HTTP server didn't receive any requests")
	}
}
