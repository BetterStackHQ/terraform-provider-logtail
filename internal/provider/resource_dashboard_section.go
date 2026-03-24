package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var dashboardSectionSchema = map[string]*schema.Schema{
	"dashboard_id": {
		Description: "The ID of the dashboard this section belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"id": {
		Description: "The ID of this section.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this section.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"y": {
		Description: "The vertical position of this section in the dashboard grid.",
		Type:        schema.TypeInt,
		Required:    true,
	},
	"collapsed": {
		Description: "Whether this section is collapsed.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"created_at": {
		Description: "The time when this section was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this section was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newDashboardSectionResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: dashboardSectionCreate,
		ReadContext:   dashboardSectionRead,
		UpdateContext: dashboardSectionUpdate,
		DeleteContext: dashboardSectionDelete,
		Importer: &schema.ResourceImporter{
			StateContext: dashboardSectionImportState,
		},
		Description: "This resource allows you to create, modify, and delete Sections in a Dashboard in Better Stack Telemetry.",
		Schema:      dashboardSectionSchema,
	}
}

type dashboardSection struct {
	Name      *string `json:"name,omitempty"`
	Y         *int    `json:"y,omitempty"`
	Collapsed *bool   `json:"collapsed,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

type dashboardSectionHTTPResponse struct {
	Data struct {
		ID         string           `json:"id"`
		Attributes dashboardSection `json:"attributes"`
	} `json:"data"`
}

func dashboardSectionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID := d.Get("dashboard_id").(string)
	in := loadDashboardSection(d)

	var out dashboardSectionHTTPResponse
	if err := resourceCreate(ctx, meta, fmt.Sprintf("/api/v2/dashboards/%s/sections", url.PathEscape(dashboardID)), &in, &out); err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%s/%s", dashboardID, out.Data.ID))
	return dashboardSectionCopyAttrs(d, &out.Data.Attributes)
}

func dashboardSectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, sectionID, err := parseDashboardSectionID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var out dashboardSectionHTTPResponse
	if diags, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(),
		fmt.Sprintf("/api/v2/dashboards/%s/sections/%s", url.PathEscape(dashboardID), url.PathEscape(sectionID)), &out); diags != nil {
		return diags
	} else if !ok {
		d.SetId("")
		return nil
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return diag.FromErr(err)
	}

	return dashboardSectionCopyAttrs(d, &out.Data.Attributes)
}

func dashboardSectionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, sectionID, err := parseDashboardSectionID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	in := loadDashboardSection(d)

	if diags := resourceUpdate(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/sections/%s", url.PathEscape(dashboardID), url.PathEscape(sectionID)), &in); diags != nil {
		return diags
	}
	return dashboardSectionRead(ctx, d, meta)
}

func dashboardSectionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, sectionID, err := parseDashboardSectionID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceDelete(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/sections/%s", url.PathEscape(dashboardID), url.PathEscape(sectionID)))
}

func dashboardSectionImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	dashboardID, sectionID, err := parseDashboardSectionID(d.Id())
	if err != nil {
		return nil, err
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return nil, err
	}

	d.SetId(fmt.Sprintf("%s/%s", dashboardID, sectionID))
	return []*schema.ResourceData{d}, nil
}

func parseDashboardSectionID(id string) (dashboardID, sectionID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid dashboard section ID format %q, expected 'dashboard_id/section_id'", id)
	}
	return parts[0], parts[1], nil
}

func loadDashboardSection(d *schema.ResourceData) dashboardSection {
	var in dashboardSection
	load(d, "name", &in.Name)
	in.Y = intFromResourceData(d, "y")
	in.Collapsed = boolFromResourceData(d, "collapsed")
	return in
}

func dashboardSectionCopyAttrs(d *schema.ResourceData, in *dashboardSection) diag.Diagnostics {
	var derr diag.Diagnostics

	if in.Name != nil {
		if err := d.Set("name", *in.Name); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Y != nil {
		if err := d.Set("y", *in.Y); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Collapsed != nil {
		if err := d.Set("collapsed", *in.Collapsed); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.CreatedAt != nil {
		if err := d.Set("created_at", *in.CreatedAt); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.UpdatedAt != nil {
		if err := d.Set("updated_at", *in.UpdatedAt); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}
