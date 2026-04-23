package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestResourceMetricUpdateMultipleFields exercises a single apply that
// changes every updatable attribute simultaneously and asserts the PATCH
// body carries all four new values while the resource ID is preserved.
func TestResourceMetricUpdateMultipleFields(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	var lastPatchBody atomic.Value
	var patchCount atomic.Int32
	id.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		prefix := "/api/v2/sources/source123/metrics"
		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, id.Load(), data.Load())))
		case r.Method == http.MethodPatch && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastPatchBody.Store(body)
			patchCount.Add(1)
			data.Store(body)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "Original"
					sql_expression = "JSONExtract(json, 'old', 'String')"
					type           = "string_low_cardinality"
					aggregations   = ["count"]
				}`,
				Check: resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
			},
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "Completely Different"
					sql_expression = "JSONExtract(json, 'new', 'Float')"
					type           = "float64_delta"
					aggregations   = ["min", "max", "p99"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "name", "Completely Different"),
					resource.TestCheckResourceAttr("logtail_metric.this", "sql_expression", "JSONExtract(json, 'new', 'Float')"),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", "float64_delta"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.#", "3"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.0", "min"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.1", "max"),
					resource.TestCheckResourceAttr("logtail_metric.this", "aggregations.2", "p99"),
					func(*terraform.State) error {
						if got := patchCount.Load(); got != 1 {
							return fmt.Errorf("expected exactly 1 PATCH in this step, got %d", got)
						}
						raw, _ := lastPatchBody.Load().([]byte)
						var got metric
						if err := json.Unmarshal(raw, &got); err != nil {
							return fmt.Errorf("PATCH body not valid JSON: %v (body=%s)", err, string(raw))
						}
						want := metric{
							Name:          strPtr("Completely Different"),
							SQLExpression: strPtr("JSONExtract(json, 'new', 'Float')"),
							Type:          strPtr("float64_delta"),
							Aggregations:  strSlicePtr("min", "max", "p99"),
						}
						if !reflect.DeepEqual(got, want) {
							return fmt.Errorf("PATCH body mismatch.\n got: %+v\nwant: %+v\nraw: %s", got, want, string(raw))
						}
						return nil
					},
				),
			},
		},
	})
}

// TestResourceMetricSourceIdForcesRecreation verifies that `source_id`
// remains ForceNew after PR #69: changing it must destroy the metric and
// create a new one, not attempt an in-place PATCH.
func TestResourceMetricSourceIdForcesRecreation(t *testing.T) {
	// State is keyed per source_id so the mock can model two independent
	// backend collections as Terraform switches between them.
	var stateBySource sync.Map // map[string]*perSourceState
	var patchCount atomic.Int32

	pathRE := regexp.MustCompile(`^/api/v2/sources/([^/]+)/metrics(?:/(\d+))?(?:\?page=\d+)?$`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		m := pathRE.FindStringSubmatch(r.RequestURI)
		if m == nil {
			t.Fatalf("unexpected path %s", r.RequestURI)
		}
		sourceID, rawID := m[1], m[2]

		stRaw, _ := stateBySource.LoadOrStore(sourceID, &perSourceState{})
		st := stRaw.(*perSourceState)

		switch {
		case r.Method == http.MethodPost && rawID == "":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			st.mu.Lock()
			st.nextID++
			st.id = st.nextID
			st.body = body
			curID := st.id
			st.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, curID, body)))
		case r.Method == http.MethodGet && rawID == "":
			st.mu.Lock()
			defer st.mu.Unlock()
			if st.id == 0 {
				_, _ = w.Write([]byte(`{"data":[],"pagination":{"next":null}}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, st.id, st.body)))
		case r.Method == http.MethodPatch && rawID != "":
			patchCount.Add(1)
			body, _ := io.ReadAll(r.Body)
			st.mu.Lock()
			st.body = body
			curID := st.id
			st.mu.Unlock()
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, curID, body)))
		case r.Method == http.MethodDelete && rawID != "":
			st.mu.Lock()
			st.id = 0
			st.body = nil
			st.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.RequestURI)
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
				resource "logtail_metric" "this" {
					source_id      = "source-a"
					name           = "Metric"
					sql_expression = "JSONExtract(json, 'x', 'String')"
					type           = "string_low_cardinality"
					aggregations   = ["count"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "source_id", "source-a"),
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
				),
			},
			{
				// Same attributes, but a different source_id — ForceNew must kick in
				// and the metric must be recreated in the new source (id resets to 1 there).
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source-b"
					name           = "Metric"
					sql_expression = "JSONExtract(json, 'x', 'String')"
					type           = "string_low_cardinality"
					aggregations   = ["count"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "source_id", "source-b"),
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					func(*terraform.State) error {
						if got := patchCount.Load(); got != 0 {
							return fmt.Errorf("expected zero PATCH calls (source_id is ForceNew), got %d", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestResourceMetricPatchErrorPropagates verifies that a non-200 PATCH
// response is surfaced to the user with URL + status code + body, rather
// than being silently swallowed.
func TestResourceMetricPatchErrorPropagates(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	id.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		prefix := "/api/v2/sources/source123/metrics"
		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, _ := io.ReadAll(r.Body)
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, id.Load(), data.Load())))
		case r.Method == http.MethodPatch && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			// 422 is not in retryablehttp's retry set, so the failure surfaces
			// immediately through resourceUpdate rather than being masked by
			// the retry middleware's "giving up" wrapper.
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":"boom"}`))
		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "M"
					sql_expression = "JSONExtract(json, 'x', 'Float')"
					type           = "float64_delta"
					aggregations   = ["avg"]
				}`,
			},
			{
				// Triggers a PATCH that the server rejects with 500.
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "M"
					sql_expression = "JSONExtract(json, 'x', 'Float')"
					type           = "float64_delta"
					aggregations   = ["p95"]
				}`,
				ExpectError: regexp.MustCompile(`PATCH .*?/metrics/1 returned 422.*boom`),
			},
		},
	})
}

// TestResourceMetricTypeFlipInPlace changes ONLY the `type` field — swapping
// between a numeric metric and a dimensional (string_low_cardinality) one.
// The provider should route this through PATCH like any other field, keeping
// the same resource ID; whether the backend can actually retype the underlying
// ClickHouse column is a separate server-side question this unit test does
// not exercise.
func TestResourceMetricTypeFlipInPlace(t *testing.T) {
	var data atomic.Value
	var id atomic.Value
	var lastPatchBody atomic.Value
	id.Store(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}
		prefix := "/api/v2/sources/source123/metrics"
		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			id.Store(id.Load().(int) + 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodGet && r.RequestURI == prefix+"?page=1":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":"%d","attributes":%s}],"pagination":{"next":null}}`, id.Load(), data.Load())))
		case r.Method == http.MethodPatch && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastPatchBody.Store(body)
			data.Store(body)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":"%d","attributes":%s}}`, id.Load(), body)))
		case r.Method == http.MethodDelete && r.RequestURI == fmt.Sprintf(`%s/%d`, prefix, id.Load()):
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	typeInBody := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			raw, _ := lastPatchBody.Load().([]byte)
			var got metric
			if err := json.Unmarshal(raw, &got); err != nil {
				return fmt.Errorf("PATCH body not valid JSON: %v (body=%s)", err, string(raw))
			}
			if got.Type == nil {
				return fmt.Errorf("PATCH body missing `type` field (body=%s)", string(raw))
			}
			if *got.Type != want {
				return fmt.Errorf("PATCH body type = %q, want %q (body=%s)", *got.Type, want, string(raw))
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
			{
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "Convertible"
					sql_expression = "JSONExtract(json, 'val', 'Float')"
					type           = "float64_delta"
					aggregations   = ["avg"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", "float64_delta"),
				),
			},
			{
				// Numeric -> dimensional. aggregations/sql_expression swapped
				// to values that make sense for a string column so the test
				// mirrors a realistic migration.
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "Convertible"
					sql_expression = "JSONExtract(json, 'val', 'String')"
					type           = "string_low_cardinality"
					aggregations   = ["uniq"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", "string_low_cardinality"),
					typeInBody("string_low_cardinality"),
				),
			},
			{
				// Dimensional -> integer numeric. Only `type` and `sql_expression`
				// differ from the previous step to keep focus on the type flip.
				Config: `
				provider "logtail" {
					api_token = "foo"
				}
				resource "logtail_metric" "this" {
					source_id      = "source123"
					name           = "Convertible"
					sql_expression = "JSONExtract(json, 'val', 'Int')"
					type           = "int64_delta"
					aggregations   = ["uniq"]
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("logtail_metric.this", "id", "1"),
					resource.TestCheckResourceAttr("logtail_metric.this", "type", "int64_delta"),
					typeInBody("int64_delta"),
				),
			},
		},
	})
}

func strPtr(s string) *string { return &s }

func strSlicePtr(vals ...string) *[]string {
	out := append([]string(nil), vals...)
	return &out
}

type perSourceState struct {
	mu     sync.Mutex
	id     int
	nextID int
	body   []byte
}
