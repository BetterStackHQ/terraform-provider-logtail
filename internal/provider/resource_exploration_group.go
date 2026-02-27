package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var explorationGroupSchema = map[string]*schema.Schema{
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
		Description: "The ID of this exploration group.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this exploration group.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"created_at": {
		Description: "The time when this exploration group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this exploration group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newExplorationGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: explorationGroupCreate,
		ReadContext:   explorationGroupRead,
		UpdateContext: explorationGroupUpdate,
		DeleteContext: explorationGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete Exploration Groups. Exploration Groups help organize explorations in Better Stack Telemetry.",
		Schema:      explorationGroupSchema,
	}
}

type explorationGroup struct {
	Name      *string `json:"name,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
}

type explorationGroupHTTPResponse struct {
	Data struct {
		ID         string           `json:"id"`
		Attributes explorationGroup `json:"attributes"`
	} `json:"data"`
}

func explorationGroupRef(in *explorationGroup) []struct {
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

func explorationGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in explorationGroup
	for _, e := range explorationGroupRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out explorationGroupHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v2/exploration-groups", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return explorationGroupCopyAttrs(d, &out.Data.Attributes)
}

func explorationGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out explorationGroupHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/exploration-groups/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return explorationGroupCopyAttrs(d, &out.Data.Attributes)
}

func explorationGroupCopyAttrs(d *schema.ResourceData, in *explorationGroup) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range explorationGroupRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func explorationGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in explorationGroup
	for _, e := range explorationGroupRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v2/exploration-groups/%s", url.PathEscape(d.Id())), &in)
}

func explorationGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/exploration-groups/%s", url.PathEscape(d.Id())))
}
