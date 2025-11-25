package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var errorsPlatformTypes = []string{
	"dot_net_errors", "dot_net_maui_errors", "dot_net_http_errors", "aiohttp_errors", "asgi_errors",
	"asp_dot_net_errors", "asp_dot_net_core_errors", "aws_lambda_dot_net_errors", "aws_lambda_node_errors",
	"aws_lambda_python_errors", "android_errors", "angular_errors", "astro_errors", "azure_functions_node_errors",
	"bottle_errors", "javascript_errors", "bun_errors", "cli_errors", "celery_errors", "chalice_errors",
	"cloudflare_pages_errors", "connect_errors", "dart_errors", "deno_errors", "django_errors", "echo_errors",
	"electron_errors", "elixir_errors", "ember_errors", "express_errors", "fasthttp_errors", "falcon_errors",
	"fastapi_errors", "fastify_errors", "fiber_errors", "flask_errors", "flutter_errors", "go_errors",
	"gatsby_errors", "gin_errors", "godot_errors", "google_cloud_function_dot_net_errors",
	"google_cloud_function_node_errors", "google_cloud_function_python_errors", "hapi_errors", "iris_errors",
	"java_errors", "koa_errors", "kotlin_errors", "laravel_errors", "log4j_errors", "logback_errors",
	"minidump_errors", "native_errors", "negroni_errors", "nest_js_errors", "next_js_errors", "node_errors",
	"nuxt_errors", "php_errors", "powershell_errors", "pyramid_errors", "qt_errors", "quart_errors",
	"rq_errors", "rack_middleware_errors", "rails_errors", "react_errors", "react_native_errors",
	"react_router_framework_errors", "remix_errors", "ruby_errors", "rust_errors", "sanic_errors",
	"serverless_python_errors", "solid_errors", "solidstart_errors", "spring_errors", "spring_boot_errors",
	"starlette_errors", "svelte_errors", "sveltekit_errors", "symfony_errors", "tanstack_start_react_errors",
	"tornado_errors", "tryton_errors", "unity_errors", "unreal_engine_errors", "vue_errors", "wpf_errors",
	"wsgi_errors", "windows_forms_errors", "ios_errors", "macos_errors",
}

var errorsApplicationSchema = map[string]*schema.Schema{
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
		Description: "The ID of this application.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "Application name. Must be unique within your team.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"token": {
		Description: "The token of this application. This token is used to identify and route the data you will send to Better Stack.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"table_name": {
		Description: "The table name generated for this application.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"platform": {
		Description: strings.ReplaceAll(fmt.Sprintf(`The platform type for the application. This helps configure appropriate SDKs and integrations. Examples: %s... Must be suffixed with `+"_errors"+`.`, strings.Join(errorsPlatformTypes[:10], ", ")), "**", "`"),
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
		ValidateDiagFunc: func(v interface{}, path cty.Path) diag.Diagnostics {
			s := v.(string)
			for _, platformType := range errorsPlatformTypes {
				if s == platformType {
					return nil
				}
			}
			return diag.Diagnostics{
				diag.Diagnostic{
					AttributePath: path,
					Severity:      diag.Error,
					Summary:       `Invalid "platform"`,
					Detail:        fmt.Sprintf("Expected one of %v", errorsPlatformTypes),
				},
			}
		},
	},
	"ingesting_host": {
		Description: "The host where the errors should be sent. See documentation for your specific platform for details.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"ingesting_paused": {
		Description: "This property allows you to temporarily pause data ingesting for this application.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"errors_retention": {
		Description:  "Error data retention period in days. Default retention is 90 days.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652), // Must be between 1 day and 10 years
	},
	"created_at": {
		Description: "The time when this application was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this application was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"data_region": {
		Description: "Data region or cluster name where application data will be stored. If omitted, the default data region for your team will be used.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"application_group_id": {
		Description: "ID of the application group this application belongs to.",
		Type:        schema.TypeString,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Treat 0 as equivalent to unset/null
			return (old == "0" || old == "") && (new == "0" || new == "")
		},
	},
}

func newErrorsApplicationResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: errorsApplicationCreate,
		ReadContext:   errorsApplicationRead,
		UpdateContext: errorsApplicationUpdate,
		DeleteContext: errorsApplicationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete your Errors applications. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications/create/",
		Schema:      errorsApplicationSchema,
	}
}

type errorsApplication struct {
	Name               *string `json:"name,omitempty"`
	Token              *string `json:"token,omitempty"`
	TableName          *string `json:"table_name,omitempty"`
	Platform           *string `json:"platform,omitempty"`
	IngestingHost      *string `json:"ingesting_host,omitempty"`
	IngestingPaused    *bool   `json:"ingesting_paused,omitempty"`
	ErrorsRetention    *int    `json:"errors_retention,omitempty"`
	CreatedAt          *string `json:"created_at,omitempty"`
	UpdatedAt          *string `json:"updated_at,omitempty"`
	TeamName           *string `json:"team_name,omitempty"`
	DataRegion         *string `json:"data_region,omitempty"`
	ApplicationGroupID *string `json:"application_group_id,omitempty"`
}

type errorsApplicationHTTPResponse struct {
	Data struct {
		ID         string            `json:"id"`
		Attributes errorsApplication `json:"attributes"`
	} `json:"data"`
}

func errorsApplicationRef(in *errorsApplication) []struct {
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
		{k: "ingesting_host", v: &in.IngestingHost},
		{k: "ingesting_paused", v: &in.IngestingPaused},
		{k: "errors_retention", v: &in.ErrorsRetention},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "data_region", v: &in.DataRegion},
		{k: "application_group_id", v: &in.ApplicationGroupID},
	}
}

func errorsApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplication
	for _, e := range errorsApplicationRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

	var out errorsApplicationHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), "/api/v1/applications", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return errorsApplicationCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out errorsApplicationHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return errorsApplicationCopyAttrs(d, &out.Data.Attributes)
}

func errorsApplicationCopyAttrs(d *schema.ResourceData, in *errorsApplication) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range errorsApplicationRef(in) {
		if e.k == "data_region" && d.Get("data_region").(string) != "" {
			// Don't update data region from API if it's already set - data_region can't change
			continue
		} else if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func errorsApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in errorsApplication
	for _, e := range errorsApplicationRef(&in) {
		if d.HasChange(e.k) {
			load(d, e.k, e.v)
		}
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())), &in)
}

func errorsApplicationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications/%s", url.PathEscape(d.Id())))
}

func newErrorsApplicationDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range errorsApplicationSchema {
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
		ReadContext: errorsApplicationLookup,
		Description: "This Data Source allows you to look up existing Errors applications using their name. For more information about the Errors API check https://betterstack.com/docs/errors/api/applications/list/",
		Schema:      s,
	}
}

type errorsApplicationPageHTTPResponse struct {
	Data []struct {
		ID         string            `json:"id"`
		Attributes errorsApplication `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func errorsApplicationLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	fetch := func(page int) (*errorsApplicationPageHTTPResponse, error) {
		res, err := meta.(*client).GetWithBaseURL(ctx, meta.(*client).ErrorsBaseURL(), fmt.Sprintf("/api/v1/applications?page=%d", page))
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
		var tr errorsApplicationPageHTTPResponse
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
				if derr := errorsApplicationCopyAttrs(d, &e.Attributes); derr != nil {
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
