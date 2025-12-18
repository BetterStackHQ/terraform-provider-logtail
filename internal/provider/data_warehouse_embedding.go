package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newWarehouseEmbeddingDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceWarehouseEmbeddingRead,
		Description: "Retrieve details of an existing Warehouse embedding configuration. Useful for accessing vector search settings and configurations for semantic similarity queries. [Learn more](https://betterstack.com/docs/warehouse/vector-embeddings/).",
		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID of the embedding to retrieve.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"source_id": {
				Description: "The ID of the warehouse source to filter embeddings by.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"embed_from": {
				Description: "The source column name containing the text to embed.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"embed_to": {
				Description: "The target column name where the generated embeddings will be stored.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"model": {
				Description: "The name of the embedding model to use.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"dimension": {
				Description: "The vector dimension of the embeddings to generate.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"created_at": {
				Description: "The time when this embedding was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"updated_at": {
				Description: "The time when this embedding was last updated.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func dataSourceWarehouseEmbeddingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	id := d.Get("id").(string)

	// If ID is provided, fetch specific embedding (requires source_id)
	if id != "" {
		if sourceID == "" {
			return diag.Errorf("source_id must be specified when looking up embedding by ID")
		}
		var singleOut warehouseEmbeddingHTTPResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/embeddings/%s", url.PathEscape(sourceID), url.PathEscape(id)), &singleOut); err != nil {
			if !ok {
				return diag.Errorf("embedding with ID %s not found", id)
			}
			return err
		}
		d.SetId(singleOut.Data.ID)
		return warehouseEmbeddingCopyAttrs(d, &singleOut.Data.Attributes)
	}

	// Otherwise, list embeddings for the specified source
	if sourceID == "" {
		return diag.Errorf("source_id must be specified to list embeddings")
	}

	fetch := func(page int) (*warehouseEmbeddingPageHTTPResponse, error) {
		params := url.Values{}
		if page > 1 {
			params.Set("page", fmt.Sprintf("%d", page))
		}

		queryString := ""
		if len(params) > 0 {
			queryString = "?" + params.Encode()
		}

		res, err := meta.(*client).do(ctx, "GET", meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/embeddings", url.PathEscape(sourceID))+queryString, nil)
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

	var allEmbeddings []struct {
		ID         string             `json:"id"`
		Attributes warehouseEmbedding `json:"attributes"`
	}
	page := 1
	for {
		pageData, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range pageData.Data {
			if sourceID == "" || *e.Attributes.EmbedFrom != "" { // Since we can't filter by embed_from directly, just collect all if no source_id filter
				allEmbeddings = append(allEmbeddings, struct {
					ID         string             `json:"id"`
					Attributes warehouseEmbedding `json:"attributes"`
				}{
					ID:         e.ID,
					Attributes: e.Attributes,
				})
			}
		}
		if pageData.Pagination.Next == "" {
			break
		}
		page++
	}

	if len(allEmbeddings) == 0 {
		return diag.Errorf("no embedding found matching the criteria")
	}
	if len(allEmbeddings) > 1 {
		return diag.Errorf("multiple embeddings found matching the criteria, please specify an ID")
	}

	embedding := allEmbeddings[0]
	d.SetId(embedding.ID)
	return warehouseEmbeddingCopyAttrs(d, &embedding.Attributes)
}
