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

// injectExplorationGroup adds fields to JSON data for exploration group testing
func injectExplorationGroup(t *testing.T, data []byte, key, value string) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = value
	result, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestResourceExplorationGroup(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v2/exploration-groups"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			// Inject server-generated fields
			body = injectExplorationGroup(t, body, "created_at", "2023-01-01T00:00:00Z")
			body = injectExplorationGroup(t, body, "updated_at", "2023-01-01T00:00:00Z")

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
			// Update the updated_at timestamp
			patched = injectExplorationGroup(t, patched, "updated_at", "2023-01-02T00:00:00Z")

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

	var name = "Test Exploration Group"

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration_group" "this" {
					name = "%s"
				}
				`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration_group.this", "name", name),
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "created_at"),
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "updated_at"),
				),
			},
			// Step 2 - update
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_exploration_group" "this" {
					name = "%s Updated"
				}
				`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "id"),
					resource.TestCheckResourceAttr("logtail_exploration_group.this", "name", name+" Updated"),
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "created_at"),
					resource.TestCheckResourceAttrSet("logtail_exploration_group.this", "updated_at"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_exploration_group.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
