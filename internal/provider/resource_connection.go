package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"slices"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var connectionSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this connection.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"client_type": {
		Description: "Type of client connection. Currently only `clickhouse` is supported.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"team_names": {
		Description: "Array of team names to associate with the connection. Only one of `team_names` or `team_ids` should be provided.",
		Type:        schema.TypeList,
		Optional:    true,
		Computed:    true,
		ForceNew:    true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	"team_ids": {
		Description: "Array of team IDs to associate with the connection. Only one of `team_names` or `team_ids` should be provided.",
		Type:        schema.TypeList,
		Optional:    true,
		Computed:    true,
		ForceNew:    true,
		Elem: &schema.Schema{
			Type: schema.TypeInt,
		},
	},
	"data_region": {
		Description: "Data region or private cluster name. Permitted values for most plans are: `us_west`, `germany`, `singapore`.",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
	},
	"ip_allowlist": {
		Description: "Array of IP addresses or CIDR ranges that are allowed to use this connection.",
		Type:        schema.TypeList,
		Optional:    true,
		ForceNew:    true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	"valid_until": {
		Description: "ISO 8601 timestamp when the connection expires.",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
	},
	"note": {
		Description: "A descriptive note for the connection.",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
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
	"password": {
		Description: "The connection password. Only available immediately after creation.",
		Type:        schema.TypeString,
		Computed:    true,
		Sensitive:   true,
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
			Schema: connectionDataSourceSchema,
		},
	},
}

var connectionDataSourceSchema = map[string]*schema.Schema{
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
}

type connection struct {
	ClientType  *string                   `json:"client_type,omitempty"`
	TeamNames   *[]string                 `json:"team_names,omitempty"`
	TeamIds     *[]int                    `json:"team_ids,omitempty"`
	DataRegion  *string                   `json:"data_region,omitempty"`
	IpAllowlist *[]string                 `json:"ip_allowlist,omitempty"`
	ValidUntil  *string                   `json:"valid_until,omitempty"`
	Note        *string                   `json:"note,omitempty"`
	Host        *string                   `json:"host,omitempty"`
	Port        *int                      `json:"port,omitempty"`
	Username    *string                   `json:"username,omitempty"`
	Password    *string                   `json:"password,omitempty"`
	CreatedAt   *string                   `json:"created_at,omitempty"`
	CreatedBy   map[string]interface{}    `json:"created_by,omitempty"`
	SampleQuery *string                   `json:"sample_query,omitempty"`
	DataSources *[]map[string]interface{} `json:"data_sources,omitempty"`
}

type connectionHTTPResponse struct {
	Data struct {
		ID         string     `json:"id"`
		Attributes connection `json:"attributes"`
	} `json:"data"`
}

func connectionRef(in *connection) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "client_type", v: &in.ClientType},
		{k: "team_names", v: &in.TeamNames},
		{k: "team_ids", v: &in.TeamIds},
		{k: "data_region", v: &in.DataRegion},
		{k: "ip_allowlist", v: &in.IpAllowlist},
		{k: "valid_until", v: &in.ValidUntil},
		{k: "note", v: &in.Note},
		{k: "host", v: &in.Host},
		{k: "port", v: &in.Port},
		{k: "username", v: &in.Username},
		{k: "password", v: &in.Password},
		{k: "created_at", v: &in.CreatedAt},
		{k: "sample_query", v: &in.SampleQuery},
	}
}

func newConnectionResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: connectionCreate,
		ReadContext:   connectionRead,
		DeleteContext: connectionDelete,
		Description:   "This resource allows you to create and manage ClickHouse connections for remote querying. For more information about the Connection API check https://betterstack.com/docs/logs/api/connections/",
		Schema:        connectionSchema,
	}
}

func connectionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in connection
	for _, e := range connectionRef(&in) {
		load(d, e.k, e.v)
	}

	var out connectionHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v1/connections", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return connectionResourceCopyAttrs(d, &out.Data.Attributes, true)
}

func connectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out connectionHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v1/connections/%s", url.PathEscape(d.Id())), &out); err != nil {
		if !ok {
			d.SetId("") // Force "create" on 404.
			return nil
		}
		return err
	}
	return connectionResourceCopyAttrs(d, &out.Data.Attributes, false)
}

func connectionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v1/connections/%s", url.PathEscape(d.Id())))
}

// connectionResourceCopyAttrs copies the API response into resource state, keeping
// the values the practitioner authored where the API would otherwise overwrite them.
//
// data_region and valid_until are configuration-only: the API echoes them normalized
// (a private cluster name resolved to its region, a reformatted timestamp) and fills
// data_region in even when none was configured. Since neither is Computed and both
// are ForceNew on a resource that has no Update, writing the API value into state
// yields a permanent diff that destroys and recreates the connection on every apply -
// so state must always keep what the configuration says.
//
// password is returned by create only; a read has none and would blank the secret.
func connectionResourceCopyAttrs(d *schema.ResourceData, in *connection, created bool) diag.Diagnostics {
	skip := []string{"data_region", "valid_until"}
	if !created {
		skip = append(skip, "password")
	}
	return connectionCopyAttrs(d, in, skip...)
}

// connectionCopyAttrs copies the API response into state. It is shared with the data
// source, which has no prior configuration to preserve and declares no `password`
// attribute; skip lists the attributes the caller sets itself or does not declare.
func connectionCopyAttrs(d *schema.ResourceData, in *connection, skip ...string) diag.Diagnostics {
	var derr diag.Diagnostics
	set := func(k string, v interface{}) {
		if slices.Contains(skip, k) {
			return
		}
		if err := d.Set(k, v); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	for _, e := range connectionRef(in) {
		set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface())
	}

	// Handle complex fields
	set("created_by", in.CreatedBy)
	in.DataSources = dropUnknownKeys(in.DataSources, connectionDataSourceSchema)
	if in.DataSources != nil && *in.DataSources != nil {
		set("data_sources", *in.DataSources)
	} else {
		set("data_sources", []interface{}{})
	}

	return derr
}
