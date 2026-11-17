package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"

	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// errorsIntegrationTestServer serves one existing integration as the errors integrations API does: GET shows it, PATCH
// changes only the settings it is sent (and only the notification toggles given), and there is no POST or DELETE.
type errorsIntegrationTestServer struct {
	*httptest.Server
	mu      sync.Mutex
	attrs   map[string]interface{}
	patches []map[string]interface{}
}

func newErrorsIntegrationTestServer(t *testing.T, path, id, attributes string) *errorsIntegrationTestServer {
	s := &errorsIntegrationTestServer{}
	if err := json.Unmarshal([]byte(attributes), &s.attrs); err != nil {
		t.Fatal(err)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		s.mu.Lock()
		defer s.mu.Unlock()

		if r.RequestURI != path+"/"+id {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
		case http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var patch map[string]interface{}
			if err := json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			var recorded map[string]interface{}
			_ = json.Unmarshal(body, &recorded)
			s.patches = append(s.patches, recorded)
			for k, v := range patch {
				if _, ok := s.attrs[k]; !ok {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(fmt.Sprintf(`{"errors":"Unknown settings: %s."}`, k)))
					return
				}
				if k == "notifications" {
					merged := s.attrs[k].(map[string]interface{})
					for name, on := range v.(map[string]interface{}) {
						merged[name] = on
					}
					continue
				}
				if k == "routes" {
					// The API answers with each channel's name.
					for _, route := range v.([]interface{}) {
						channels := []interface{}{}
						for _, c := range route.(map[string]interface{})["channels"].([]interface{}) {
							channels = append(channels, map[string]interface{}{"id": c, "name": "Channel " + c.(string)})
						}
						route.(map[string]interface{})["channels"] = channels
					}
				}
				s.attrs[k] = v
			}
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
		out, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{"id": id, "type": "integration", "attributes": s.attrs}})
		_, _ = w.Write(out)
	}))
	return s
}

func (s *errorsIntegrationTestServer) takePatches() []map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	patches := s.patches
	s.patches = nil
	return patches
}

func errorsIntegrationProviders(url string) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"logtail": func() (*schema.Provider, error) {
			return New(WithURL(url)), nil
		},
	}
}

// expectPatches checks the PATCH bodies sent since the last check, as JSON.
func expectPatches(s *errorsIntegrationTestServer, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		got := s.takePatches()
		if len(got) != len(want) {
			encoded, _ := json.Marshal(got)
			return fmt.Errorf("expected %d PATCH requests, got %d: %s", len(want), len(got), encoded)
		}
		for i := range want {
			var expected interface{}
			if err := json.Unmarshal([]byte(want[i]), &expected); err != nil {
				return err
			}
			if !jsonEqual(mustJSON(got[i]), expected) {
				return fmt.Errorf("PATCH %d: expected %s, got %s", i, want[i], mustJSON(got[i]))
			}
		}
		return nil
	}
}

func mustJSON(v interface{}) string {
	out, _ := json.Marshal(v)
	return string(out)
}

const linearIntegrationAttributes = `{
	"team_id": "t123", "team_name": "Engineering",
	"applications": [{"application_id": 11, "environments": ["production"]}],
	"notifications": {"spikes": false, "new_errors": true, "automatically_reopened": false, "occurrence_milestones": false, "occurrence_threshold": false, "affected_users_threshold": false},
	"occurrence_threshold_count": 10, "occurrence_threshold_interval": "minute", "affected_users_threshold_count": 10,
	"tag_filter": [{"tag": "environment", "values": ["production"]}],
	"name": "Errors", "linear_team_id": "team-1", "linear_assignee_id": "user-1", "linear_status_id": "", "linear_resolved_status_id": "",
	"linear_project_id": "", "linear_label_ids": ["label-b", "label-a"], "linear_priority": "urgent",
	"auto_reopen": true, "auto_resolve": true, "sync_comments": true
}`

func TestResourceErrorsLinearIntegration(t *testing.T) {
	server := newErrorsIntegrationTestServer(t, "/api/v1/linear-integrations", "63418", linearIntegrationAttributes)
	defer server.Close()
	name := "logtail_errors_linear_integration.this"

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: errorsIntegrationProviders(server.URL),
		CheckDestroy:      expectPatches(server),
		Steps: []resource.TestStep{
			// Adopting without settings changes nothing and reads every setting back.
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_linear_integration" "this" {
					integration_id = 63418
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server),
					resource.TestCheckResourceAttr(name, "id", "63418"),
					resource.TestCheckResourceAttr(name, "team_name", "Engineering"),
					resource.TestCheckResourceAttr(name, "name", "Errors"),
					resource.TestCheckResourceAttr(name, "applications.#", "1"),
					resource.TestCheckResourceAttr(name, "applications.0.application_id", "11"),
					resource.TestCheckResourceAttr(name, "notifications.0.new_errors", "true"),
					resource.TestCheckResourceAttr(name, "linear_label_ids.#", "2"),
					resource.TestCheckResourceAttr(name, "tag_filter.0.tag", "environment"),
					resource.TestCheckResourceAttr(name, "auto_resolve", "true"),
				),
			},
			// Only the configured settings are sent, and only the configured toggles within notifications.
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_linear_integration" "this" {
					integration_id = 63418
					linear_priority = "high"
					auto_resolve = false
					notifications {
						spikes = true
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"linear_priority": "high", "auto_resolve": false, "notifications": {"spikes": true}}`),
					resource.TestCheckResourceAttr(name, "linear_priority", "high"),
					resource.TestCheckResourceAttr(name, "auto_resolve", "false"),
					resource.TestCheckResourceAttr(name, "notifications.0.spikes", "true"),
					resource.TestCheckResourceAttr(name, "notifications.0.new_errors", "true"),
				),
			},
			// Explicitly empty values clear a setting; settings left out of the configuration stay as they are.
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_linear_integration" "this" {
					integration_id = 63418
					linear_priority = "high"
					auto_resolve = false
					notifications {
						spikes = true
					}
					applications = []
					tag_filter = []
					linear_label_ids = []
					linear_assignee_id = ""
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"applications": [], "tag_filter": [], "linear_label_ids": [], "linear_assignee_id": ""}`),
					resource.TestCheckResourceAttr(name, "applications.#", "0"),
					resource.TestCheckResourceAttr(name, "tag_filter.#", "0"),
					resource.TestCheckResourceAttr(name, "linear_label_ids.#", "0"),
					resource.TestCheckResourceAttr(name, "linear_assignee_id", ""),
					resource.TestCheckResourceAttr(name, "name", "Errors"),
				),
			},
			// Lists of objects and thresholds.
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_linear_integration" "this" {
					integration_id = 63418
					linear_priority = "high"
					auto_resolve = false
					notifications {
						spikes = true
						occurrence_threshold = true
						new_errors = false
					}
					occurrence_threshold_count = 50
					occurrence_threshold_interval = "hour"
					applications = [
						{ application_id = 12, environments = ["production", "staging"] },
						{ application_id = 11, environments = [] },
					]
					tag_filter = [{ tag = "region", values = ["eu", "us"] }]
					linear_label_ids = ["label-a"]
					linear_assignee_id = ""
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{
						"notifications": {"spikes": true, "occurrence_threshold": true, "new_errors": false},
						"occurrence_threshold_count": 50, "occurrence_threshold_interval": "hour",
						"applications": [{"application_id": 11, "environments": []}, {"application_id": 12, "environments": ["production", "staging"]}],
						"tag_filter": [{"tag": "region", "values": ["eu", "us"]}],
						"linear_label_ids": ["label-a"]
					}`),
					resource.TestCheckResourceAttr(name, "applications.#", "2"),
					resource.TestCheckResourceAttr(name, "occurrence_threshold_count", "50"),
					resource.TestCheckResourceAttr(name, "notifications.0.new_errors", "false"),
					resource.TestCheckResourceAttr(name, "tag_filter.0.values.1", "us"),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Removing the resource sends nothing: the integration stays connected (CheckDestroy expects no PATCH).
		},
	})
}

func TestResourceErrorsIntegrationNotFound(t *testing.T) {
	server := newErrorsIntegrationTestServer(t, "/api/v1/jira-integrations", "1", `{}`)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: errorsIntegrationProviders(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_jira_integration" "this" {
					integration_id = 2
				}`,
				ExpectError: regexp.MustCompile(`Jira integration 2 was not found`),
			},
		},
	})
}

func TestResourceErrorsSlackIntegration(t *testing.T) {
	server := newErrorsIntegrationTestServer(t, "/api/v1/slack-integrations", "7", `{
		"team_id": "t123", "team_name": "Engineering", "applications": [],
		"notifications": {"spikes": true, "spike_protection": true, "new_releases": true, "new_errors": true, "occurrence_milestones": true, "occurrence_threshold": false, "affected_users_threshold": false, "automatically_reopened": false},
		"occurrence_threshold_count": 10, "occurrence_threshold_interval": "minute", "affected_users_threshold_count": 10, "tag_filter": [],
		"slack_team_name": "Acme", "slack_channel_name": "#errors", "post_updates": "single_channel", "broadcast_updates": true, "ai_sre_on_mention": false
	}`)
	defer server.Close()
	name := "logtail_errors_slack_integration.this"

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: errorsIntegrationProviders(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_slack_integration" "this" {
					integration_id = 7
					post_updates = "thread_based"
					applications = [{ application_id = 11, environments = ["production"] }]
					notifications {
						new_releases = false
					}
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"post_updates": "thread_based", "applications": [{"application_id": 11, "environments": ["production"]}], "notifications": {"new_releases": false}}`),
					resource.TestCheckResourceAttr(name, "slack_channel_name", "#errors"),
					resource.TestCheckResourceAttr(name, "post_updates", "thread_based"),
					resource.TestCheckResourceAttr(name, "notifications.0.new_releases", "false"),
					resource.TestCheckResourceAttr(name, "notifications.0.spikes", "true"),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestResourceErrorsMicrosoftTeamsIntegration(t *testing.T) {
	server := newErrorsIntegrationTestServer(t, "/api/v1/microsoft-teams-integrations", "8", `{
		"team_id": "t123", "team_name": "Engineering",
		"notifications": {"spikes": true, "spike_protection": true, "new_releases": true, "new_errors": true, "occurrence_milestones": true, "occurrence_threshold": false, "affected_users_threshold": false, "automatically_reopened": false},
		"occurrence_threshold_count": 10, "occurrence_threshold_interval": "minute", "affected_users_threshold_count": 10, "tag_filter": [],
		"microsoft_teams_team_name": "Acme", "microsoft_teams_channel_name": "General", "routes": []
	}`)
	defer server.Close()
	name := "logtail_errors_microsoft_teams_integration.this"

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: errorsIntegrationProviders(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_microsoft_teams_integration" "this" {
					integration_id = 8
					routes = [{
						applications = [{ application_id = 11, environments = ["production"] }]
						channel_ids  = ["19:abc@thread.tacv2"]
					}]
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"routes": [{"applications": [{"application_id": 11, "environments": ["production"]}], "channels": ["19:abc@thread.tacv2"]}]}`),
					resource.TestCheckResourceAttr(name, "microsoft_teams_channel_name", "General"),
					resource.TestCheckResourceAttr(name, "routes.#", "1"),
					resource.TestCheckResourceAttr(name, "routes.0.channel_ids.0", "19:abc@thread.tacv2"),
				),
			},
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_microsoft_teams_integration" "this" {
					integration_id = 8
					routes = []
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"routes": []}`),
					resource.TestCheckResourceAttr(name, "routes.#", "0"),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestResourceErrorsJiraIntegration(t *testing.T) {
	server := newErrorsIntegrationTestServer(t, "/api/v1/jira-integrations", "9", `{
		"team_id": "t123", "team_name": "Engineering", "applications": [],
		"notifications": {"spikes": false, "new_errors": true, "automatically_reopened": false, "occurrence_milestones": false, "occurrence_threshold": false, "affected_users_threshold": false},
		"occurrence_threshold_count": 10, "occurrence_threshold_interval": "minute", "affected_users_threshold_count": 10, "tag_filter": [],
		"name": "Errors", "jira_site_url": "https://acme.atlassian.net", "jira_project_key": "OPS", "jira_issue_type_id": "10001",
		"jira_fields": {"priority": {"id": "2"}}
	}`)
	defer server.Close()
	name := "logtail_errors_jira_integration.this"

	resource.Test(t, resource.TestCase{
		IsUnitTest:        true,
		ProviderFactories: errorsIntegrationProviders(server.URL),
		Steps: []resource.TestStep{
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_jira_integration" "this" {
					integration_id = 9
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server),
					resource.TestCheckResourceAttr(name, "jira_site_url", "https://acme.atlassian.net"),
					resource.TestCheckResourceAttr(name, "jira_fields", `{"priority":{"id":"2"}}`),
				),
			},
			// Formatting of jira_fields never shows as a change; its content does.
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_jira_integration" "this" {
					integration_id = 9
					jira_fields = jsonencode({ priority = { id = "3" }, labels = ["errors"] })
				}`,
				Check: resource.ComposeTestCheckFunc(
					expectPatches(server, `{"jira_fields": {"priority": {"id": "3"}, "labels": ["errors"]}}`),
				),
			},
			{
				Config: `
				provider "logtail" { api_token = "foo" }
				resource "logtail_errors_jira_integration" "this" {
					integration_id = 9
					jira_fields = <<-EOT
					  { "labels": ["errors"], "priority": { "id": "3" } }
					EOT
				}`,
				PlanOnly: true,
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
