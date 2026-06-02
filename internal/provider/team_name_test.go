package provider

import (
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
// value fails with a helpful error. team_name is only used when creating a resource with a
// global token.
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
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, data.Load().([]byte))))
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
}
