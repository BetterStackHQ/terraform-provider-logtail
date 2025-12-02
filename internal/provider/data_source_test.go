package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataMonitor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/sources"

		switch {
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(`{"data":[{"id":"1","attributes":{"name":"Test Source","token":"token123","table_name":"abc", "team_id": 123456,"platform":"ubuntu"}}],"pagination":{"next":"..."}}`))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=2":
			_, _ = w.Write([]byte(`{"data":[{"id":"2","attributes":{"name":"Other Test Source","token":"token456","table_name":"def", "team_id": 123456,"platform":"ubuntu"}}],"pagination":{"next":null}}`))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var table_name = "def"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_source" "this" {
					table_name = "%s"
				}
				`, table_name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.logtail_source.this", "id"),
					resource.TestCheckResourceAttr("data.logtail_source.this", "table_name", table_name),
					resource.TestCheckResourceAttr("data.logtail_source.this", "platform", "ubuntu"),
					resource.TestCheckResourceAttr("data.logtail_source.this", "team_id", "123456"),
				),
			},
		},
	})
}

func TestDataSourceGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/source-groups"

		switch {
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(`{"data":[{"id":"1","attributes":{"name":"Test Group","sort_index":1,"created_at":"2023-01-01T00:00:00Z","updated_at":"2023-01-01T00:00:00Z"}}],"pagination":{"next":"` + prefix + `?page=2"}}`))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=2":
			_, _ = w.Write([]byte(`{"data":[{"id":"2","attributes":{"name":"Production Group","sort_index":2,"created_at":"2023-01-02T00:00:00Z","updated_at":"2023-01-02T00:00:00Z"}}],"pagination":{"next":null}}`))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var groupName = "Production Group"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_source_group" "this" {
					name = "%s"
				}
				`, groupName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.logtail_source_group.this", "id"),
					resource.TestCheckResourceAttr("data.logtail_source_group.this", "name", groupName),
					resource.TestCheckResourceAttr("data.logtail_source_group.this", "sort_index", "2"),
					resource.TestCheckResourceAttrSet("data.logtail_source_group.this", "created_at"),
					resource.TestCheckResourceAttrSet("data.logtail_source_group.this", "updated_at"),
				),
			},
		},
	})
}

func TestDataSourceGroupNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/source-groups"

		switch {
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(`{"data":[{"id":"1","attributes":{"name":"Test Group","sort_index":1}}],"pagination":{"next":null}}`))
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}

				data "logtail_source_group" "this" {
					name = "Nonexistent Group"
				}
				`,
				ExpectError: regexp.MustCompile(`Source group with name "Nonexistent Group" not found`),
			},
		},
	})
}

// TODO: test duplicate
