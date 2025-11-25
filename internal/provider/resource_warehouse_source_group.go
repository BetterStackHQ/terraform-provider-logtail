package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var warehouseSourceGroupSchema = map[string]*schema.Schema{
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
		Description: "The ID of this warehouse source group.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of the warehouse source group. Can contain letters, numbers, spaces, and special characters.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"created_at": {
		Description: "The time when this warehouse source group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this warehouse source group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"sort_index": {
		Description: "The sort index of this warehouse source group.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
}

func newWarehouseSourceGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: warehouseSourceGroupCreate,
		ReadContext:   warehouseSourceGroupRead,
		UpdateContext: warehouseSourceGroupUpdate,
		DeleteContext: warehouseSourceGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete your Warehouse source groups. For more information about the Warehouse API check https://betterstack.com/docs/warehouse/api/source-groups/create/",
		Schema:      warehouseSourceGroupSchema,
	}
}

type warehouseSourceGroup struct {
	Name      *string `json:"name,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
	SortIndex *int    `json:"sort_index,omitempty"`
}

type warehouseSourceGroupHTTPResponse struct {
	Data struct {
		ID         string               `json:"id"`
		Attributes warehouseSourceGroup `json:"attributes"`
	} `json:"data"`
}

func warehouseSourceGroupRef(in *warehouseSourceGroup) []struct {
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
		{k: "sort_index", v: &in.SortIndex},
	}
}

func warehouseSourceGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseSourceGroup
	for _, e := range warehouseSourceGroupRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out warehouseSourceGroupHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), "/api/v1/source-groups", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return warehouseSourceGroupCopyAttrs(d, &out.Data.Attributes)
}

func warehouseSourceGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out warehouseSourceGroupHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return warehouseSourceGroupCopyAttrs(d, &out.Data.Attributes)
}

func warehouseSourceGroupCopyAttrs(d *schema.ResourceData, in *warehouseSourceGroup) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range warehouseSourceGroupRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func warehouseSourceGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseSourceGroup
	for _, e := range warehouseSourceGroupRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())), &in)
}

func warehouseSourceGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/source-groups/%s", url.PathEscape(d.Id())))
}

func newWarehouseSourceGroupDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range warehouseSourceGroupSchema {
		cp := *v
		switch k {
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
		}
		s[k] = &cp
	}
	return &schema.Resource{
		ReadContext: warehouseSourceGroupLookup,
		Description: "This Data Source allows you to look up existing Warehouse source groups using their name. For more information about the Warehouse API check https://betterstack.com/docs/warehouse/api/source-groups/index/",
		Schema:      s,
	}
}

type warehouseSourceGroupPageHTTPResponse struct {
	Data []struct {
		ID         string               `json:"id"`
		Attributes warehouseSourceGroup `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func warehouseSourceGroupLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*warehouseSourceGroupPageHTTPResponse, error) {
		res, err := meta.(*client).GetWithBaseURL(ctx, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/source-groups?page=%d", page))
		if err != nil {
			return nil, err
		}
		defer func() {
			// Keep-Alive.
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}()
		body, err := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
		}
		if err != nil {
			return nil, err
		}
		var tr warehouseSourceGroupPageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}
	name := d.Get("name").(string)
	page := 1
	for {
		res, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range res.Data {
			if *e.Attributes.Name == name {
				if d.Id() != "" {
					return diag.Errorf("duplicate")
				}
				d.SetId(e.ID)
				if derr := warehouseSourceGroupCopyAttrs(d, &e.Attributes); derr != nil {
					return derr
				}
			}
		}
		page++
		if res.Pagination.Next == "" {
			return nil
		}
	}
}
