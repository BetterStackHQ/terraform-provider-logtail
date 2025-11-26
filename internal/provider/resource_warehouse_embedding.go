package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var warehouseEmbeddingSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this embedding.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"source_id": {
		Description: "The ID of the Warehouse source to create the embedding for.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"embed_from": {
		Description: "The source column name containing the text to embed.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"embed_to": {
		Description: "The target column name where the generated embeddings will be stored.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"model": {
		Description: "The name of the embedding model to use (e.g., `embeddinggemma:300m`).",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"dimension": {
		Description: "The vector dimension of the embeddings to generate.",
		Type:        schema.TypeInt,
		Required:    true,
		ForceNew:    true,
	},
	"created_at": {
		Description: "The time when this embedding was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this embedding was last updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newWarehouseEmbeddingResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: warehouseEmbeddingCreate,
		ReadContext:   warehouseEmbeddingRead,
		DeleteContext: warehouseEmbeddingDelete,
		Description:   "This resource allows you to create and manage embeddings for vector similarity search in Warehouse. For more information about the Warehouse Embeddings API check https://betterstack.com/docs/warehouse/api/embeddings/",
		Schema:        warehouseEmbeddingSchema,
	}
}

type warehouseEmbedding struct {
	EmbedFrom *string `json:"embed_from,omitempty"`
	EmbedTo   *string `json:"embed_to,omitempty"`
	Model     *string `json:"model,omitempty"`
	Dimension *int    `json:"dimension,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

type warehouseEmbeddingHTTPResponse struct {
	Data struct {
		ID         string             `json:"id"`
		Attributes warehouseEmbedding `json:"attributes"`
	} `json:"data"`
}

type warehouseEmbeddingPageHTTPResponse struct {
	Data []struct {
		ID         string             `json:"id"`
		Attributes warehouseEmbedding `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func warehouseEmbeddingRef(in *warehouseEmbedding) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "embed_from", v: &in.EmbedFrom},
		{k: "embed_to", v: &in.EmbedTo},
		{k: "model", v: &in.Model},
		{k: "dimension", v: &in.Dimension},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
	}
}

func warehouseEmbeddingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseEmbedding
	for _, e := range warehouseEmbeddingRef(&in) {
		load(d, e.k, e.v)
	}

	sourceID := d.Get("source_id").(string)

	var out warehouseEmbeddingHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/embeddings", url.PathEscape(sourceID)), &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return warehouseEmbeddingCopyAttrs(d, &out.Data.Attributes)
}

func warehouseEmbeddingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)

	// List embeddings and find the one with matching ID
	fetch := func(page int) (*warehouseEmbeddingPageHTTPResponse, error) {
		res, err := meta.(*client).do(ctx, "GET", meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/embeddings?page=%d", url.PathEscape(sourceID), page), nil)
		if err != nil {
			return nil, err
		}
		defer func() {
			_, _ = io.ReadAll(res.Body)
			_ = res.Body.Close()
		}()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
		}
		var pageOut warehouseEmbeddingPageHTTPResponse
		if err := json.Unmarshal(body, &pageOut); err != nil {
			return nil, err
		}
		return &pageOut, nil
	}

	page := 1
	for {
		pageData, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range pageData.Data {
			if e.ID == d.Id() {
				return warehouseEmbeddingCopyAttrs(d, &e.Attributes)
			}
		}
		if pageData.Pagination.Next == "" {
			break
		}
		page++
	}

	// If we get here, the embedding was not found
	d.SetId("") // Force "create" on 404.
	return nil
}

func warehouseEmbeddingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/embeddings/%s", url.PathEscape(sourceID), url.PathEscape(d.Id())))
}

func warehouseEmbeddingCopyAttrs(d *schema.ResourceData, in *warehouseEmbedding) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range warehouseEmbeddingRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}
