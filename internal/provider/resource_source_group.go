package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sourceGroupSchema = map[string]*schema.Schema{
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
		Description: "The ID of this source group.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this source group.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"sort_index": {
		Description: "The sort index of this source group.",
		Type:        schema.TypeInt,
		Optional:    true,
	},
	"created_at": {
		Description: "The time when this source group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this source group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newSourceGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceGroupCreate,
		ReadContext:   sourceGroupRead,
		UpdateContext: sourceGroupUpdate,
		DeleteContext: sourceGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Organize log sources into logical groups for better management. Group related sources together to apply shared configurations, transformations, and access controls across multiple log streams. [Learn more](https://betterstack.com/docs/logs/api/listing-sources-in-source-group/).",
		Schema:      sourceGroupSchema,
	}
}

type sourceGroup struct {
	Name      *string `json:"name,omitempty"`
	SortIndex *int    `json:"sort_index,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
}

type sourceGroupHTTPResponse struct {
	Data struct {
		ID         string      `json:"id"`
		Attributes sourceGroup `json:"attributes"`
	} `json:"data"`
}

func sourceGroupRef(in *sourceGroup) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "sort_index", v: &in.SortIndex},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
	}
}

func sourceGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in sourceGroup
	for _, e := range sourceGroupRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out sourceGroupHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v1/source-groups", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return sourceGroupCopyAttrs(d, &out.Data.Attributes)
}

func sourceGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceGroupHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return sourceGroupCopyAttrs(d, &out.Data.Attributes)
}

func sourceGroupCopyAttrs(d *schema.ResourceData, in *sourceGroup) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range sourceGroupRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func sourceGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in sourceGroup
	for _, e := range sourceGroupRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())), &in)
}

func sourceGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())))
}
