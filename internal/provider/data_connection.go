package provider

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// connectionLookupOmits lists the resource attributes the lookup neither declares
// nor reads back: password is only ever returned when a connection is created, so a
// lookup has nothing to return. Keep it the single source of truth - a schema that
// declares an attribute the read skips (or vice versa) is what made
// connectionCopyAttrs panic in the first place. Not to be confused with
// connectionDataSourceSchema, which describes the `data_sources` attribute.
var connectionLookupOmits = []string{"password"}

func newConnectionDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range connectionSchema {
		if slices.Contains(connectionLookupOmits, k) {
			continue
		}
		cp := *v
		switch k {
		case "id":
			// ID is used for lookup - make it settable.
			cp.Description = "The ID of the connection to retrieve."
			cp.Optional = true
			cp.Computed = true
			cp.Required = false
		default:
			// All other fields become computed (read-only). Clear every
			// resource-only flag, so one added to connectionSchema later cannot
			// leak in here - a computed-only attribute has no user input to
			// validate, default, or constrain.
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ForceNew = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.ConflictsWith = nil
			cp.MaxItems = 0
			cp.MinItems = 0
		}
		s[k] = &cp
	}

	return &schema.Resource{
		ReadContext: dataSourceConnectionRead,
		Description: "This data source allows you to retrieve information about ClickHouse connections. For more information about the Connection API check https://betterstack.com/docs/logs/api/connections/",
		Schema:      s,
	}
}

func dataSourceConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)

	// If ID is provided, fetch specific connection
	if id != "" {
		var singleOut connectionHTTPResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/connections/%s", url.PathEscape(id)), &singleOut); err != nil {
			if !ok {
				return diag.Errorf("connection with ID %s not found", id)
			}
			return err
		}
		d.SetId(singleOut.Data.ID)
		return connectionCopyAttrs(d, &singleOut.Data.Attributes, connectionLookupOmits...)
	}

	// Otherwise, list connections (this would return all, which might be too many)
	return diag.Errorf("connection data source requires an ID to be specified")
}
