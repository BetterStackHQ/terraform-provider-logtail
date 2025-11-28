package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestResourceWarehouseEmbedding(t *testing.T) {
	var data atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("Received " + r.Method + " " + r.RequestURI)

		if r.Header.Get("Authorization") != "Bearer foo" {
			t.Fatal("Not authorized: " + r.Header.Get("Authorization"))
		}

		prefix := "/api/v1/sources/ws-123/embeddings"
		id := "1"

		switch {
		case r.Method == http.MethodPost && r.RequestURI == prefix:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = inject(t, body, "created_at", "2025-01-26T09:40:00.000Z")
			body = inject(t, body, "updated_at", "2025-01-26T09:40:00.000Z")

			data.Store(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"id":%q,"attributes":%s}}`, id, body)))
		case r.Method == http.MethodGet && (r.RequestURI == prefix || r.RequestURI == prefix+"?page=1"):
			// Return list of embeddings
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":%q,"attributes":%s}],"pagination":{"first":"1","last":"1","prev":null,"next":null}}`, id, data.Load().([]byte))))
		case r.Method == http.MethodDelete && r.RequestURI == prefix+"/"+id:
			w.WriteHeader(http.StatusNoContent)
			data.Store([]byte(nil))
		default:
			t.Fatal("Unexpected " + r.Method + " " + r.RequestURI)
		}
	}))
	defer server.Close()

	var warehouseSourceID = "ws-123"
	var embedFrom = "message.text"
	var embedTo = "message.embedding"
	var model = "embeddinggemma:300m"
	var dimension = 512

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"logtail": func() (*schema.Provider, error) {
				return New(WithURL(server.URL)), nil
			},
		},
		Steps: []resource.TestStep{
			// Step 1 - create.
			{
				Config: fmt.Sprintf(`
				provider "logtail" {
					api_token = "foo"
				}

				resource "logtail_warehouse_embedding" "this" {
					source_id = "%s"
					embed_from         = "%s"
					embed_to           = "%s"
					model              = "%s"
					dimension          = %d
				}
				`, warehouseSourceID, embedFrom, embedTo, model, dimension),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("logtail_warehouse_embedding.this", "id"),
					resource.TestCheckResourceAttr("logtail_warehouse_embedding.this", "source_id", warehouseSourceID),
					resource.TestCheckResourceAttr("logtail_warehouse_embedding.this", "embed_from", embedFrom),
					resource.TestCheckResourceAttr("logtail_warehouse_embedding.this", "embed_to", embedTo),
					resource.TestCheckResourceAttr("logtail_warehouse_embedding.this", "model", model),
					resource.TestCheckResourceAttr("logtail_warehouse_embedding.this", "dimension", "512"),
				),
			},
		},
	})
}
