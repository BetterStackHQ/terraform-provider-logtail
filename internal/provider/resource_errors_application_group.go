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

var errorsApplicationGroupSchema = map[string]*schema.Schema{
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
		Description: "The ID of this application group.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "Application group name. Must be unique within your team.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"created_at": {
		Description: "The time when this application group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this application group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"sort_index": {
		Description: "The sort index of this application group.",
		Type:        schema.TypeInt,
		Optional:    false,
		Computed:    true,
	},
}

func newErrorsApplicationGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: errorsApplicationGroupCreate,
		ReadContext:   errorsApplicationGroupRead,
		UpdateContext: errorsApplicationGroupUpdate,
		DeleteContext: errorsApplicationGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete your Errors application groups. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications-groups/create/",
		Schema:      errorsApplicationGroupSchema,
	}
}

type errorsApplicationGroup struct {
	Name      *string `json:"name,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
	SortIndex *int    `json:"sort_index,omitempty"`
}

type errorsApplicationGroupHTTPResponse struct {
	Data struct {
		ID         string                 `json:"id"`
		Attributes errorsApplicationGroup `json:"attributes"`
	} `json:"data"`
}

func errorsApplicationGroupRef(in *errorsApplicationGroup) []struct {
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

func errorsApplicationGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplicationGroup
	for _, e := range errorsApplicationGroupRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out errorsApplicationGroupHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), "/api/v1/application-groups", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return errorsApplicationGroupCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out errorsApplicationGroupHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/application-groups/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return errorsApplicationGroupCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationGroupCopyAttrs(d *schema.ResourceData, in *errorsApplicationGroup) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range errorsApplicationGroupRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func errorsApplicationGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplicationGroup
	for _, e := range errorsApplicationGroupRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/application-groups/%s", url.PathEscape(d.Id())), &in)
}

func errorsApplicationGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/application-groups/%s", url.PathEscape(d.Id())))
}

func newErrorsApplicationGroupDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range errorsApplicationGroupSchema {
		cp := *v
		switch k {
		case "name":
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		case "platform":
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.Default = nil
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.ForceNew = false
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
		ReadContext: errorsApplicationGroupLookup,
		Description: "This Data Source allows you to look up existing Errors application groups using their name. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications-groups/list/",
		Schema:      s,
	}
}

type errorsApplicationGroupPageHTTPResponse struct {
	Data []struct {
		ID         string                 `json:"id"`
		Attributes errorsApplicationGroup `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func errorsApplicationGroupLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*errorsApplicationGroupPageHTTPResponse, error) {
		res, err := meta.(*client).GetWithBaseURL(ctx, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/application-groups?page=%d", page))
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
		var tr errorsApplicationGroupPageHTTPResponse
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
				if derr := errorsApplicationGroupCopyAttrs(d, &e.Attributes); derr != nil {
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
