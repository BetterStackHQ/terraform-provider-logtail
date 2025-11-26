package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newConnectionDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceConnectionRead,
		Description: "This data source allows you to retrieve information about ClickHouse connections. For more information about the Connection API check https://betterstack.com/docs/logs/api/connections/",
		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID of the connection to retrieve.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"client_type": {
				Description: "Type of client connection.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"team_names": {
				Description: "Array of team names associated with the connection.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"team_ids": {
				Description: "Array of team IDs associated with the connection.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
			"data_region": {
				Description: "Data region of the connection.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"ip_allowlist": {
				Description: "Array of IP addresses allowed to use this connection.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"valid_until": {
				Description: "Timestamp when the connection expires.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"note": {
				Description: "Descriptive note for the connection.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"host": {
				Description: "The connection hostname.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"port": {
				Description: "The connection port.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"username": {
				Description: "The connection username.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_at": {
				Description: "The time when this connection was created.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"created_by": {
				Description: "Information about the user who created this connection.",
				Type:        schema.TypeMap,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"sample_query": {
				Description: "A sample query showing how to use this connection.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"data_sources": {
				Description: "List of available data sources for this connection.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"source_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"source_id": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"team_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"data_sources": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)

	// If ID is provided, fetch specific connection
	if id != "" {
		var singleOut connectionHTTPResponse
		if err, ok := resourceRead(ctx, meta, fmt.Sprintf("/api/v1/connections/%s", url.PathEscape(id)), &singleOut); err != nil {
			if !ok {
				return diag.Errorf("connection with ID %s not found", id)
			}
			return err
		}
		d.SetId(singleOut.Data.ID)
		return connectionCopyAttrs(d, &singleOut.Data.Attributes)
	}

	// Otherwise, list connections (this would return all, which might be too many)
	return diag.Errorf("connection data source requires an ID to be specified")
}
