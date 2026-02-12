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
)

func TestResourceCollector(t *testing.T) {
	var collectorData atomic.Value       // Main collector data (without databases)
	var databasesData atomic.Value       // Databases stored separately
	databasesData.Store([]interface{}{}) // Initialize with empty slice

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/collectors"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "secret", "generated_secret_token")
			body = inject(t, body, "status", "active")
			body = inject(t, body, "team_id", 123456)
			body = inject(t, body, "source_id", 42)
			body = inject(t, body, "hosts_count", 0)
			body = inject(t, body, "hosts_up_count", 0)

			// Handle custom_bucket - remove secret_access_key from response
			body = removeCustomBucketSecret(t, body)

			// Handle HTTP Basic Auth - move enable flag to configuration and remove password
			body = processProxyConfig(t, body)

			// Extract databases and store separately, replace with databases_count in main response
			body, databases := extractDatabasesFromResponse(t, body)
			databasesData.Store(databases)

			collectorData.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id+"/databases":
			// Return databases in the expected format
			databases := databasesData.Load().([]interface{})
			_, _ = w.Write([]byte(formatDatabasesResponse(t, databases)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, collectorData.Load().([]byte))))

		case r.Method == http.MethodPatch && r.RequestURI == prefix+"/"+id:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(collectorData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patched, err := json.Marshal(patch)
			if err != nil {
				t.Fatal(err)
			}
			patched = inject(t, patched, "secret", "generated_secret_token")
			patched = inject(t, patched, "status", "active")
			patched = inject(t, patched, "team_id", 123456)
			patched = inject(t, patched, "source_id", 42)

			// Handle custom_bucket - remove secret_access_key from response
			patched = removeCustomBucketSecret(t, patched)

			// Handle HTTP Basic Auth - move enable flag to configuration and remove password
			patched = processProxyConfig(t, patched)

			// Handle databases update - process _destroy, then extract and store separately
			patched = processDatabasesUpdate(t, patched)
			patched, databases := extractDatabasesFromResponse(t, patched)
			databasesData.Store(databases)

			collectorData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			collectorData.Store([]byte(nil))
			databasesData.Store([]interface{}{})

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var name = "Test Collector"
	var platform = "docker"

	// Test basic CRUD operations
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					note     = "Test collector for unit tests"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "name", name),
					resource.TestCheckResourceAttr("logtail_collector.this", "platform", platform),
					resource.TestCheckResourceAttr("logtail_collector.this", "team_id", "123456"),
					resource.TestCheckResourceAttr("logtail_collector.this", "secret", "generated_secret_token"),
					resource.TestCheckResourceAttr("logtail_collector.this", "status", "active"),
					resource.TestCheckResourceAttr("logtail_collector.this", "note", "Test collector for unit tests"),
					resource.TestCheckResourceAttr("logtail_collector.this", "source_id", "42"),
				),
			},
			// Step 2 - update
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name              = "%s"
					platform          = "%s"
					note              = "Updated note"
					logs_retention    = 30
					metrics_retention = 90
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "name", name),
					resource.TestCheckResourceAttr("logtail_collector.this", "note", "Updated note"),
					resource.TestCheckResourceAttr("logtail_collector.this", "logs_retention", "30"),
					resource.TestCheckResourceAttr("logtail_collector.this", "metrics_retention", "90"),
				),
			},
			// Step 3 - import
			{
				ResourceName:      "logtail_collector.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})

	// Test configuration nested block with components
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with configuration
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						logs_sample_rate   = 100
						traces_sample_rate = 50

						components {
							ebpf_metrics = true
							logs_host    = true
							logs_docker  = true
							metrics_nginx = true
						}
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.logs_sample_rate", "100"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.traces_sample_rate", "50"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.ebpf_metrics", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.logs_host", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.logs_docker", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.metrics_nginx", "true"),
				),
			},
			// Step 2 - update configuration
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						logs_sample_rate   = 75
						traces_sample_rate = 25

						components {
							ebpf_metrics      = false
							logs_host         = true
							ebpf_tracing_full = true
						}
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.logs_sample_rate", "75"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.traces_sample_rate", "25"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.ebpf_metrics", "false"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.ebpf_tracing_full", "true"),
				),
			},
		},
	})

	// Test custom_bucket
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-collector-bucket"
						endpoint          = "https://s3.amazonaws.com"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.name", "my-collector-bucket"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.endpoint", "https://s3.amazonaws.com"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.access_key_id", "AKIAIOSFODNN7EXAMPLE"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.secret_access_key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.keep_data_after_retention", "false"),
				),
			},
			// Step 2 - update with custom_bucket still present (secret should be preserved)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					note     = "Updated with bucket"
					custom_bucket {
						name              = "my-collector-bucket"
						endpoint          = "https://s3.amazonaws.com"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "note", "Updated with bucket"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.0.secret_access_key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				),
			},
		},
	})

	// Test custom_bucket removal validation
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-bucket"
						endpoint          = "https://s3.amazonaws.com"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.#", "1"),
				),
			},
			// Step 2 - try to remove custom_bucket (should fail)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket cannot be removed once set`),
			},
		},
	})

	// Test data_region immutability
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with data_region
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name        = "%s"
					platform    = "%s"
					data_region = "us_east"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "data_region", "us_east"),
				),
			},
			// Step 2 - try to change data_region (should fail)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name        = "%s"
					platform    = "%s"
					data_region = "germany"
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`data_region cannot be changed after collector is created`),
			},
		},
	})

	// Test proxy_config rejected for non-proxy platforms
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_buffering_proxy = true
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`proxy_config is only applicable to proxy platform collectors`),
			},
		},
	})

	// Test databases
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with database
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					databases {
						service_type = "postgres"
						host         = "db.example.com"
						port         = 5432
						username     = "collector"
						password     = "secret123"
						ssl_mode     = "require"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.service_type", "postgres"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.host", "db.example.com"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.port", "5432"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.username", "collector"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.password", "secret123"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.ssl_mode", "require"),
				),
			},
			// Step 2 - add another database
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					databases {
						service_type = "postgres"
						host         = "db.example.com"
						port         = 5432
						username     = "collector"
						password     = "secret123"
						ssl_mode     = "require"
					}

					databases {
						service_type = "redis"
						host         = "redis.example.com"
						port         = 6379
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.#", "2"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.service_type", "postgres"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.1.service_type", "redis"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.1.host", "redis.example.com"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.1.port", "6379"),
				),
			},
			// Step 3 - remove first database, keep second
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					databases {
						service_type = "redis"
						host         = "redis.example.com"
						port         = 6379
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.service_type", "redis"),
				),
			},
		},
	})

	// Test custom_bucket modification validation (fields cannot be changed once set)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with custom_bucket
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "my-bucket"
						endpoint          = "https://s3.amazonaws.com"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "custom_bucket.#", "1"),
				),
			},
			// Step 2 - try to modify custom_bucket fields (should fail)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"
					custom_bucket {
						name              = "different-bucket"
						endpoint          = "https://s3.amazonaws.com"
						access_key_id     = "AKIAIOSFODNN7EXAMPLE"
						secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
					}
				}
				`, name, platform),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`custom_bucket fields cannot be modified after creation`),
			},
		},
	})

	// Test MongoDB database
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					databases {
						service_type = "mongodb"
						host         = "mongo.example.com"
						port         = 27017
						username     = "admin"
						password     = "mongopass"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.service_type", "mongodb"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.host", "mongo.example.com"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.port", "27017"),
				),
			},
		},
	})

	// Test MySQL database with TLS
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					databases {
						service_type = "mysql"
						host         = "mysql.example.com"
						port         = 3306
						username     = "root"
						password     = "mysqlpass"
						tls          = "true"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.service_type", "mysql"),
					resource.TestCheckResourceAttr("logtail_collector.this", "databases.0.tls", "true"),
				),
			},
		},
	})
}

func TestResourceCollectorSwarmPlatform(t *testing.T) {
	var collectorData atomic.Value
	var databasesData atomic.Value
	databasesData.Store([]interface{}{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/collectors"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "secret", "swarm_secret")
			body = inject(t, body, "status", "active")
			body = inject(t, body, "team_id", 123456)
			body = inject(t, body, "source_id", 55)
			body = inject(t, body, "hosts_count", 0)
			body = inject(t, body, "hosts_up_count", 0)
			body, databases := extractDatabasesFromResponse(t, body)
			databasesData.Store(databases)
			collectorData.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id+"/databases":
			databases := databasesData.Load().([]interface{})
			_, _ = w.Write([]byte(formatDatabasesResponse(t, databases)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, collectorData.Load().([]byte))))

		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			collectorData.Store([]byte(nil))
			databasesData.Store([]interface{}{})

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

				resource "logtail_collector" "this" {
					name     = "Swarm Collector"
					platform = "swarm"
					note     = "Docker Swarm cluster"

					configuration {
						logs_sample_rate = 100
						components {
							logs_docker = true
							logs_host   = true
						}
					}
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "platform", "swarm"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.logs_sample_rate", "100"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.logs_docker", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.components.0.logs_host", "true"),
				),
			},
		},
	})
}

func TestDataSourceCollector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/collectors" {
			name := r.URL.Query().Get("name")
			if name == "My Collector" {
				_, _ = w.Write([]byte(`{
					"data": [{
						"id": "42",
						"attributes": {
							"name": "My Collector",
							"platform": "kubernetes",
							"status": "active",
							"secret": "lookup_secret",
							"source_id": 99,
							"team_id": 999,
							"hosts_count": 5,
							"hosts_up_count": 3
						}
					}],
					"pagination": {"next": ""}
				}`))
			} else {
				_, _ = w.Write([]byte(`{"data": [], "pagination": {"next": ""}}`))
			}
			return
		}

		t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
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

				data "logtail_collector" "this" {
					name = "My Collector"
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.logtail_collector.this", "id", "42"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "name", "My Collector"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "platform", "kubernetes"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "status", "active"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "secret", "lookup_secret"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "source_id", "99"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "team_id", "999"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "hosts_count", "5"),
					resource.TestCheckResourceAttr("data.logtail_collector.this", "hosts_up_count", "3"),
				),
			},
		},
	})

	// Test not found
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

				data "logtail_collector" "this" {
					name = "Nonexistent Collector"
				}
				`,
				ExpectError: regexp.MustCompile(`collector with name "Nonexistent Collector" not found`),
			},
		},
	})
}

// extractDatabasesFromResponse extracts databases from the response body,
// replaces them with databases_count, and returns both the modified body and the databases.
// This simulates the API behavior where the main collector response includes databases_count
// but not the actual databases array.
func extractDatabasesFromResponse(t *testing.T, body json.RawMessage) (json.RawMessage, []interface{}) {
	response := make(map[string]interface{})
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}

	var databases []interface{}
	if dbs, ok := response["databases"].([]interface{}); ok {
		// Process databases: remove passwords and assign IDs
		for i, db := range dbs {
			if dbMap, ok := db.(map[string]interface{}); ok {
				delete(dbMap, "password")
				// Assign ID if not present
				if _, hasID := dbMap["id"]; !hasID {
					dbMap["id"] = float64(i + 1) // Use float64 for JSON number
				}
				databases = append(databases, dbMap)
			}
		}
		// Replace databases array with databases_count in the response
		response["databases_count"] = len(databases)
		delete(response, "databases")
	} else {
		// No databases - set count to 0
		response["databases_count"] = 0
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	return body, databases
}

// formatDatabasesResponse formats databases into the expected API response format.
// The /databases endpoint returns: {"data": [{"id": 1, "attributes": {...}}, ...]}
func formatDatabasesResponse(t *testing.T, databases []interface{}) string {
	var dataItems []string
	for _, db := range databases {
		dbMap := db.(map[string]interface{})
		id := dbMap["id"]

		// Create a copy of the map for attributes (excluding id)
		attrs := make(map[string]interface{})
		for k, v := range dbMap {
			if k != "id" {
				attrs[k] = v
			}
		}

		attrsJSON, err := json.Marshal(attrs)
		if err != nil {
			t.Fatal(err)
		}
		// Format ID as string (JSONAPI serializer returns string IDs)
		idInt := int(id.(float64))
		dataItems = append(dataItems, fmt.Sprintf(`{"id":"%d","attributes":%s}`, idInt, string(attrsJSON)))
	}

	if len(dataItems) == 0 {
		return `{"data":[]}`
	}
	return fmt.Sprintf(`{"data":[%s]}`, strings.Join(dataItems, ","))
}

// processDatabasesUpdate handles the PATCH request for databases.
// It processes _destroy flags and assigns IDs to new databases.
func processDatabasesUpdate(t *testing.T, body json.RawMessage) json.RawMessage {
	response := make(map[string]interface{})
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}

	if databases, ok := response["databases"].([]interface{}); ok {
		var result []interface{}
		nextID := 100 // Use higher IDs for newly added databases
		for _, db := range databases {
			if dbMap, ok := db.(map[string]interface{}); ok {
				// Skip databases marked for destruction
				if destroy, ok := dbMap["_destroy"].(bool); ok && destroy {
					continue
				}
				delete(dbMap, "password")
				delete(dbMap, "_destroy")
				// Assign ID if not present
				if _, hasID := dbMap["id"]; !hasID {
					dbMap["id"] = float64(nextID)
					nextID++
				}
				result = append(result, dbMap)
			}
		}
		response["databases"] = result
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	return body
}

// processProxyConfig handles proxy_config fields.
// The API nests all proxy settings under proxy_config in the response.
// http_basic_auth_password is removed (write-only field, never returned by the API).
func processProxyConfig(t *testing.T, body json.RawMessage) json.RawMessage {
	response := make(map[string]interface{})
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}

	// If proxy_config exists, remove password from it
	if proxyConfig, ok := response["proxy_config"].(map[string]interface{}); ok {
		delete(proxyConfig, "http_basic_auth_password")
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	return body
}

func TestResourceCollectorNewFeatures(t *testing.T) {
	var collectorData atomic.Value
	var databasesData atomic.Value
	databasesData.Store([]interface{}{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/collectors"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "secret", "generated_secret_token")
			body = inject(t, body, "status", "active")
			body = inject(t, body, "team_id", 123456)
			body = inject(t, body, "source_id", 42)
			body = inject(t, body, "hosts_count", 0)
			body = inject(t, body, "hosts_up_count", 0)
			body = removeCustomBucketSecret(t, body)
			body = processProxyConfig(t, body)
			body, databases := extractDatabasesFromResponse(t, body)
			databasesData.Store(databases)
			collectorData.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id+"/databases":
			databases := databasesData.Load().([]interface{})
			_, _ = w.Write([]byte(formatDatabasesResponse(t, databases)))

		case r.Method == http.MethodGet && r.RequestURI == prefix+"/"+id:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, collectorData.Load().([]byte))))

		case r.Method == http.MethodPatch && r.RequestURI == prefix+"/"+id:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			patch := make(map[string]interface{})
			if err = json.Unmarshal(collectorData.Load().([]byte), &patch); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(body, &patch); err != nil {
				t.Fatal(err)
			}
			patched, err := json.Marshal(patch)
			if err != nil {
				t.Fatal(err)
			}
			patched = inject(t, patched, "secret", "generated_secret_token")
			patched = inject(t, patched, "status", "active")
			patched = inject(t, patched, "team_id", 123456)
			patched = inject(t, patched, "source_id", 42)
			patched = removeCustomBucketSecret(t, patched)
			patched = processProxyConfig(t, patched)
			patched = processDatabasesUpdate(t, patched)
			patched, databases := extractDatabasesFromResponse(t, patched)
			databasesData.Store(databases)
			collectorData.Store(patched)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, patched)))

		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			collectorData.Store([]byte(nil))
			databasesData.Store([]interface{}{})

		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var name = "Test Collector"
	var platform = "proxy"

	// Test VRL transformation
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with VRL transformation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						vrl_transformation = ".level = downcase!(.level)"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.vrl_transformation", ".level = downcase!(.level)"),
				),
			},
			// Step 2 - update VRL transformation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						vrl_transformation = ".processed = true"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.vrl_transformation", ".processed = true"),
				),
			},
		},
	})

	// Test SSL/TLS certificate settings via proxy_config
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with SSL settings
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_ssl_certificate = true
						ssl_certificate_host   = "logs.example.com"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_ssl_certificate", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.ssl_certificate_host", "logs.example.com"),
				),
			},
			// Step 2 - update SSL settings
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_ssl_certificate = true
						ssl_certificate_host   = "logs2.example.com"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.ssl_certificate_host", "logs2.example.com"),
				),
			},
		},
	})

	// Test HTTP Basic Auth via proxy_config
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with proxy_config
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_http_basic_auth   = true
						http_basic_auth_username = "api_user"
						http_basic_auth_password = "secret_password"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_http_basic_auth", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_username", "api_user"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_password", "secret_password"),
				),
			},
			// Step 2 - update username (password preserved)
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_http_basic_auth   = true
						http_basic_auth_username = "new_user"
						http_basic_auth_password = "secret_password"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_http_basic_auth", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_username", "new_user"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_password", "secret_password"),
				),
			},
			// Step 3 - disable HTTP Basic Auth
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_http_basic_auth = false
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_http_basic_auth", "false"),
				),
			},
		},
	})

	// Test user_vector_config
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with user_vector_config
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					user_vector_config = "sources:\n  custom_input:\n    type: file\n"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "user_vector_config", "sources:\n  custom_input:\n    type: file\n"),
				),
			},
			// Step 2 - update user_vector_config
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					user_vector_config = "sources:\n  updated_input:\n    type: file\n"
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "user_vector_config", "sources:\n  updated_input:\n    type: file\n"),
				),
			},
		},
	})

	// Test batching (in configuration) and buffering proxy (in proxy_config)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with batching config and buffering proxy
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						disk_batch_size_mb   = 512
						memory_batch_size_mb = 20
					}

					proxy_config {
						enable_buffering_proxy    = true
						buffering_proxy_listen_on = "0.0.0.0:8080"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.disk_batch_size_mb", "512"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.memory_batch_size_mb", "20"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_buffering_proxy", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.buffering_proxy_listen_on", "0.0.0.0:8080"),
				),
			},
			// Step 2 - update batching config and disable buffering proxy
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						disk_batch_size_mb   = 1024
						memory_batch_size_mb = 30
					}

					proxy_config {
						enable_buffering_proxy = false
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.disk_batch_size_mb", "1024"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.memory_batch_size_mb", "30"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_buffering_proxy", "false"),
				),
			},
		},
	})

	// Test ingesting_paused
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with ingesting_paused = true
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = true
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "ingesting_paused", "true"),
				),
			},
			// Step 2 - update to unpause
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name             = "%s"
					platform         = "%s"
					ingesting_paused = false
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "ingesting_paused", "false"),
				),
			},
		},
	})

	// Test when_full
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with when_full = block
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						when_full = "block"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.when_full", "block"),
				),
			},
			// Step 2 - update to drop_newest
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						when_full = "drop_newest"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.when_full", "drop_newest"),
				),
			},
		},
	})

	// Test service_option and namespace_option
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create with service_option and namespace_option
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						service_option {
							name          = "my-service"
							log_sampling  = 50
							ingest_traces = false
						}

						namespace_option {
							name          = "staging"
							log_sampling  = 10
							ingest_traces = true
						}
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.service_option.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("logtail_collector.this", "configuration.0.service_option.*", map[string]string{
						"name": "my-service", "log_sampling": "50", "ingest_traces": "false",
					}),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.namespace_option.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("logtail_collector.this", "configuration.0.namespace_option.*", map[string]string{
						"name": "staging", "log_sampling": "10", "ingest_traces": "true",
					}),
				),
			},
			// Step 2 - update service_option and add another namespace_option
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					configuration {
						service_option {
							name          = "my-service"
							log_sampling  = 100
							ingest_traces = true
						}

						namespace_option {
							name         = "production"
							log_sampling = 100
						}

						namespace_option {
							name         = "staging"
							log_sampling = 25
						}
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.service_option.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("logtail_collector.this", "configuration.0.service_option.*", map[string]string{
						"name": "my-service", "log_sampling": "100", "ingest_traces": "true",
					}),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.namespace_option.#", "2"),
				),
			},
		},
	})

	// Test combined features (proxy_config has SSL + basic auth + buffering, configuration has sampling + VRL)
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					proxy_config {
						enable_http_basic_auth    = true
						http_basic_auth_username  = "proxy_user"
						http_basic_auth_password  = "proxy_pass"
						enable_ssl_certificate    = true
						ssl_certificate_host      = "logs.example.com"
						enable_buffering_proxy    = true
						buffering_proxy_listen_on = "0.0.0.0:80"
					}

					configuration {
						logs_sample_rate   = 100
						traces_sample_rate = 50
						vrl_transformation = ".level = downcase!(.level)"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_http_basic_auth", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_username", "proxy_user"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.http_basic_auth_password", "proxy_pass"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_ssl_certificate", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.ssl_certificate_host", "logs.example.com"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.enable_buffering_proxy", "true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "proxy_config.0.buffering_proxy_listen_on", "0.0.0.0:80"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.#", "1"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.logs_sample_rate", "100"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.traces_sample_rate", "50"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.vrl_transformation", ".level = downcase!(.level)"),
				),
			},
		},
	})

	// Test source_vrl_transformation (server-side VRL) — create, update, remove
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1: Create with source_vrl_transformation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					source_vrl_transformation = ".environment = \"production\""
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "source_vrl_transformation", ".environment = \"production\""),
				),
			},
			// Step 2: Update source_vrl_transformation
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					source_vrl_transformation = ".environment = \"staging\""
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_collector.this", "source_vrl_transformation", ".environment = \"staging\""),
				),
			},
			// Step 3: Import
			{
				ResourceName:      "logtail_collector.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})

	// Test both on-host VRL and server-side VRL together
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

				resource "logtail_collector" "this" {
					name     = "%s"
					platform = "%s"

					source_vrl_transformation = ".enriched = true"

					configuration {
						vrl_transformation = ".level = downcase!(.level)"
					}
				}
				`, name, platform),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_collector.this", "id"),
					resource.TestCheckResourceAttr("logtail_collector.this", "source_vrl_transformation", ".enriched = true"),
					resource.TestCheckResourceAttr("logtail_collector.this", "configuration.0.vrl_transformation", ".level = downcase!(.level)"),
				),
			},
		},
	})
}
