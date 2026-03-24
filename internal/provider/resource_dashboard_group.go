package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var dashboardGroupSchema = map[string]*schema.Schema{
	"team_name": {
		Description: "Used to specify the team the resource should be created in when using global tokens.",
		Type:        schema.TypeString,
		Optional:    true,
		Default:     nil,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			return d.Id() != ""
		},
	},
	"id": {
		Description: "The ID of this dashboard group.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this dashboard group.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"created_at": {
		Description: "The time when this dashboard group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this dashboard group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newDashboardGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: dashboardGroupCreate,
		ReadContext:   dashboardGroupRead,
		UpdateContext: dashboardGroupUpdate,
		DeleteContext: dashboardGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete Dashboard Groups. Dashboard Groups help organize dashboards in Better Stack Telemetry.",
		Schema:      dashboardGroupSchema,
	}
}

type dashboardGroup struct {
	Name      *string `json:"name,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
}

type dashboardGroupHTTPResponse struct {
	Data struct {
		ID         string         `json:"id"`
		Attributes dashboardGroup `json:"attributes"`
	} `json:"data"`
}

func dashboardGroupRef(in *dashboardGroup) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
	}
}

func dashboardGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in dashboardGroup
	for _, e := range dashboardGroupRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out dashboardGroupHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v2/dashboard-groups", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return dashboardGroupCopyAttrs(d, &out.Data.Attributes)
}

func dashboardGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out dashboardGroupHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboard-groups/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return dashboardGroupCopyAttrs(d, &out.Data.Attributes)
}

func dashboardGroupCopyAttrs(d *schema.ResourceData, in *dashboardGroup) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range dashboardGroupRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func dashboardGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in dashboardGroup
	for _, e := range dashboardGroupRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v2/dashboard-groups/%s", url.PathEscape(d.Id())), &in)
}

func dashboardGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/dashboard-groups/%s", url.PathEscape(d.Id())))
}
