package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var databaseExporterComponents = []string{"metrics_postgres", "metrics_mysql", "metrics_elasticsearch", "metrics_pgbouncer"}

// The API keeps a database exporter it isn't sent (on for a new collector, as stored on update),
// and a metrics_databases sent without metrics_cluster also sets the exporters left out.
func TestResourceCollectorLeavesUnsetDatabaseExportersToTheAPI(t *testing.T) {
	var mu sync.Mutex
	stored := map[string]interface{}{}
	var sent []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodPost || r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var req struct {
				Configuration struct {
					Components map[string]interface{} `json:"components"`
				} `json:"configuration"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatal(err)
			}
			components := req.Configuration.Components
			sent = append(sent, components)

			legacy, hasLegacy := components["metrics_databases"]
			cluster, hasCluster := components["metrics_cluster"]
			for k, v := range components {
				stored[k] = v
			}
			if hasCluster {
				stored["metrics_databases"] = cluster
			} else if hasLegacy {
				stored["metrics_cluster"] = legacy
			}
			for _, key := range databaseExporterComponents {
				if _, ok := components[key]; ok {
					continue
				}
				if hasLegacy && !hasCluster {
					stored[key] = legacy
				} else if r.Method == http.MethodPost {
					stored[key] = true
				}
			}
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
			}
			writeCollector(t, w, stored)
		case r.Method == http.MethodGet && r.RequestURI == "/api/v1/collectors/1/databases":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodGet:
			writeCollector(t, w, stored)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	config := func(components string) string {
		return fmt.Sprintf(`
			provider "logtail" {
				api_token = "foo"
			}

			resource "logtail_collector" "this" {
				name     = "Components collector"
				platform = "docker"
				configuration {
					components {
						%s
					}
				}
			}
		`, components)
	}
	lastSent := func() map[string]interface{} {
		mu.Lock()
		defer mu.Unlock()
		return sent[len(sent)-1]
	}
	notSent := func(keys ...string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			for _, key := range keys {
				if v, ok := lastSent()[key]; ok {
					return fmt.Errorf("%s was sent as %v although the configuration leaves it out", key, v)
				}
			}
			return nil
		}
	}
	sentAs := func(key string, want bool) resource.TestCheckFunc {
		return func(*terraform.State) error {
			if got, ok := lastSent()[key]; !ok || got != want {
				return fmt.Errorf("%s was sent as %v, want %v", key, got, want)
			}
			return nil
		}
	}
	exporters := func(want string) resource.TestCheckFunc {
		var checks []resource.TestCheckFunc
		for _, key := range databaseExporterComponents {
			checks = append(checks, resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0."+key, want))
		}
		return resource.ComposeTestCheckFunc(checks...)
	}

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) { return New(WithURL(server.URL)), nil },
		},
		Steps: []resource.TestStep{
			{
				Config: config(`logs_docker = true`),
				Check:  resource.ComposeTestCheckFunc(notSent(databaseExporterComponents...), exporters("true")),
			},
			{
				Config: config("logs_docker = true\nmetrics_postgres = false"),
				Check: resource.ComposeTestCheckFunc(
					sentAs("metrics_postgres", false),
					notSent("metrics_mysql", "metrics_elasticsearch", "metrics_pgbouncer"),
				),
			},
			{
				Config: config("logs_docker = true\nmetrics_databases = true"),
				Check: resource.ComposeTestCheckFunc(
					sentAs("metrics_databases", true),
					notSent(append([]string{"metrics_cluster"}, databaseExporterComponents...)...),
				),
			},
			{
				Config: config("logs_docker = true\nmetrics_databases = false"),
				Check: resource.ComposeTestCheckFunc(
					sentAs("metrics_databases", false),
					notSent(append([]string{"metrics_cluster"}, databaseExporterComponents...)...),
				),
			},
			{
				Config: config("logs_docker = true\nmetrics_cluster = true\nmetrics_databases = true"),
				Check:  resource.ComposeTestCheckFunc(sentAs("metrics_cluster", true), sentAs("metrics_databases", true)),
			},
		},
	})
}

func writeCollector(t *testing.T, w http.ResponseWriter, components map[string]interface{}) {
	attrs, err := json.Marshal(map[string]interface{}{
		"name": "Components collector", "platform": "docker", "secret": "generated_secret_token", "status": "active",
		"team_id": 123456, "source_id": 42, "hosts_count": 0, "hosts_up_count": 0,
		"configuration": map[string]interface{}{"components": components},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"1","attributes":%s}}`, attrs)))
}
