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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestTeamNameCannotBeChangedAfterCreate verifies team_name handling after a resource exists:
// clearing it is a silent no-op (no plan change), while changing it to a different, non-empty
// value fails with a helpful error. When state has no team_name (created without one, or
// imported), Read fills in the team reported by the API, so a matching value is accepted and a
// different one is rejected. team_name is only used when creating a resource with a global token.
func TestTeamNameCannotBeChangedAfterCreate(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/source-groups"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			// Like the API, report the resource's actual team, whether or not it was sent on create.
			var attrs map[string]interface{}
			if err := json.Unmarshal(data.Load().([]byte), &attrs); err != nil {
				t.Fatal(err)
			}
			attrs["team_name"] = "First team"
			body, err := json.Marshal(attrs)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	withTeamName := func(teamName string) string {
		return fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_group" "this" {
					name      = "Test Source Group"
					team_name = "%s"
				}
				`, teamName)
	}
	withoutTeamName := `
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_source_group" "this" {
					name = "Test Source Group"
				}
				`

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create in a team.
			{
				Config: withTeamName("First team"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_group.this", "team_name", "First team"),
				),
			},
			// Step 2 - clearing team_name is a no-op: no plan change, no error.
			{
				Config:   withoutTeamName,
				PlanOnly: true,
			},
			// Step 3 - changing team_name to a different team must fail.
			{
				Config:      withTeamName("Second team"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`team_name cannot be changed after resource is created`),
			},
		},
	})

	// A resource created without team_name has none in state; refresh fills in the actual team, so
	// adding the matching team_name later (e.g. a module that started passing it for new
	// environments) is accepted, while a different team is still rejected.
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: withoutTeamName,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("logtail_source_group.this", "team_name"),
				),
			},
			{
				Config:   withTeamName("First team"),
				PlanOnly: true,
			},
			{
				Config:      withTeamName("Second team"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`team_name cannot be changed after resource is created`),
			},
			{
				Config: withoutTeamName,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_source_group.this", "team_name", "First team"),
				),
			},
		},
	})
}
