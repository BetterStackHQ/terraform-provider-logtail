package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newDashboardSectionDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)

	for k, v := range dashboardSectionSchema {
		cp := *v
		switch k {
		case "dashboard_id":
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
			cp.ForceNew = false
		case "name":
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		default:
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.ForceNew = false
		}
		s[k] = &cp
	}

	return &schema.Resource{
		ReadContext: dashboardSectionLookup,
		Description: "This data source allows you to get information about a Section in a Dashboard in Better Stack Telemetry.",
		Schema:      s,
	}
}

type dashboardSectionsHTTPResponse struct {
	Data []struct {
		ID         string           `json:"id"`
		Attributes dashboardSection `json:"attributes"`
	} `json:"data"`
}

func dashboardSectionLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID := d.Get("dashboard_id").(string)
	name := d.Get("name").(string)
	c := meta.(*client)

	res, err := c.Get(ctx, fmt.Sprintf("/api/v2/dashboards/%s/sections", url.PathEscape(dashboardID)))
	if err != nil {
		return diag.FromErr(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	body, err := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return diag.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
	}
	if err != nil {
		return diag.FromErr(err)
	}

	var out dashboardSectionsHTTPResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return diag.FromErr(err)
	}

	for _, item := range out.Data {
		if item.Attributes.Name != nil && *item.Attributes.Name == name {
			d.SetId(fmt.Sprintf("%s/%s", dashboardID, item.ID))

			if err := d.Set("dashboard_id", dashboardID); err != nil {
				return diag.FromErr(err)
			}

			return dashboardSectionCopyAttrs(d, &item.Attributes)
		}
	}

	return diag.Errorf("Section with name %q not found in dashboard %s", name, dashboardID)
}
