package provider

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var platformTypes = []string{"kubernetes", "docker", "ruby", "python", "javascript", "node", "logstash", "fluentbit", "fluentd", "rsyslog", "syslog-ng", "http", "vector", "heroku", "heroku_addon", "ubuntu", "apache2", "nginx", "postgresql", "mysql", "mongodb", "redis", "cloudflare_worker", "flights", "dokku", "fly_io"}

var sourceSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this source.",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"name": {
		Description: "The name of this source.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"token": {
		Description: "The token of this source. This token is used to identify and route the data you will send to Logtail.",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"table_name": {
		Description: "The table name generated for this source.",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"platform": {
		Description: strings.ReplaceAll(`The platform of this source. This value can be set only when you're creating a new source. You can't update this value later. Valid values are:
    **kubernetes**
	**docker**
	**ruby**
	**python**
	**javascript**
	**node**
	**logstash**
	**fluentbit**
	**fluentd**
	**rsyslog**
	**syslog-ng**
	**http**
	**vector**
	**heroku**
	**heroku_addon**
	**ubuntu**
	**apache2**
	**nginx**
	**postgresql**
	**mysql**
	**mongodb**
	**redis**
	**cloudflare_worker**
	**flights**
	**dokku**
	**fly_io**`,
	"**", "`"),
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
		ValidateDiagFunc: func(v interface{}, path cty.Path) diag.Diagnostics {
			s := v.(string)
			for _, platformType := range platformTypes {
				if s == platformType {
					return nil
				}
			}
			return diag.Diagnostics{
				diag.Diagnostic{
					AttributePath: path,
					Severity:      diag.Error,
					Summary:       `Invalid "platform"`,
					Detail:        fmt.Sprintf("Expected one of %v", platformTypes),
				},
			}
		},
	},
	"ingesting_paused": {
		Description: "This proparty allows you to temporarily pause data ingesting for this source (e.g., when you are reaching your plan's usage quota and you want to prioritize some sources over others).",
		Type:        schema.TypeBool,
		Optional:    true,
	},
}

func newSourceResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceCreate,
		ReadContext:   sourceRead,
		UpdateContext: sourceUpdate,
		DeleteContext: sourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete Logtail Sources. For more information about the Sources API check https://docs.logtail.com/api/sources-api",
		Schema:      sourceSchema,
	}
}

type source struct {
	Name            *string `json:"name,omitempty"`
	Token           *string `json:"token,omitempty"`
	TableName       *string `json:"table_name,omitempty"`
	Platform        *string `json:"platform,omitempty"`
	IngestingPaused *bool   `json:"ingesting_paused,omitempty"`
}

type sourceHTTPResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes source `json:"attributes"`
	} `json:"data"`
}

func sourceRef(in *source) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "token", v: &in.Token},
		{k: "table_name", v: &in.TableName},
		{k: "platform", v: &in.Platform},
		{k: "ingesting_paused", v: &in.IngestingPaused},
	}
}

func sourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in source
	for _, e := range sourceRef(&in) {
		load(d, e.k, e.v)
	}
	var out sourceHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v1/sources", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return sourceCopyAttrs(d, &out.Data.Attributes)
}

func sourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceHTTPResponse
	if err, ok := resourceRead(ctx, meta, fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return sourceCopyAttrs(d, &out.Data.Attributes)
}

func sourceCopyAttrs(d *schema.ResourceData, in *source) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range sourceRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func sourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in source
	for _, e := range sourceRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())), &in)
}

func sourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v1/sources/%s", url.PathEscape(d.Id())))
}
