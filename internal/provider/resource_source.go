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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var platformTypes = []string{"apache2", "aws_cloudwatch", "aws_ecs", "aws_elb", "aws_fargate", "cloudflare_logpush", "cloudflare_worker", "datadog_agent", "digitalocean", "docker", "dokku", "dotnet", "elasticsearch", "erlang", "filebeat", "fluentbit", "fluentd", "fly_io", "go", "haproxy", "heroku", "http", "java", "javascript", "kubernetes", "logstash", "minio", "mongodb", "mysql", "nginx", "open_telemetry", "php", "postgresql", "prometheus", "prometheus_scrape", "python", "rabbitmq", "redis", "render", "rsyslog", "ruby", "syslog-ng", "traefik", "ubuntu", "vector", "vercel_integration"}

var sourceSchema = map[string]*schema.Schema{
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
		Description: "The ID of this source.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this source.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"token": {
		Description: "The token of this source. This token is used to identify and route the data you will send to Better Stack.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"table_name": {
		Description: "The table name generated for this source.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"platform": {
		Description: strings.ReplaceAll(`The platform of this source. This value can be set only when you're creating a new source. You can't update this value later. Valid values are:
    - **apache2**
    - **aws_cloudwatch**
    - **aws_ecs**
    - **aws_elb**
    - **aws_fargate**
    - **cloudflare_logpush**
    - **cloudflare_worker**
    - **datadog_agent**
    - **digitalocean**
    - **docker**
    - **dokku**
    - **dotnet**
    - **elasticsearch**
    - **erlang**
    - **filebeat**
    - **flights**
    - **fluentbit**
    - **fluentd**
    - **fly_io**
    - **go**
    - **haproxy**
    - **heroku**
    - **http**
    - **java**
    - **javascript**
    - **kubernetes**
    - **logstash**
    - **minio**
    - **mongodb**
    - **mysql**
    - **nginx**
    - **open_telemetry**
    - **php**
    - **postgresql**
    - **prometheus**
    - **prometheus_scrape**
    - **python**
    - **rabbitmq**
    - **redis**
    - **render**
    - **rsyslog**
    - **ruby**
    - **syslog-ng**
    - **traefik**
    - **ubuntu**
    - **vector**
    - **vercel_integration**`, "**", "`"),
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
	"ingesting_host": {
		Description: "The host where the logs or metrics should be sent. See [documentation](https://betterstack.com/docs/logs/start/) for your specific source platform for details.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"ingesting_paused": {
		Description: "This property allows you to temporarily pause data ingesting for this source (e.g., when you are reaching your plan's usage quota and you want to prioritize some sources over others).",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"logs_retention": {
		Description:  "Data retention for logs in days. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652), // Must be between 1 day and 10 years
	},
	"metrics_retention": {
		Description:  "Data retention for metrics in days. There might be additional charges for longer retention.",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 3652), // Must be between 1 day and 10 years
	},
	"live_tail_pattern": {
		Description: "Freeform text template for formatting Live tail output with columns wrapped in {column} brackets. Example: \"PID: {message_json.pid} {level} {message}\"",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"created_at": {
		Description: "The time when this monitor group was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this monitor group was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"scrape_urls": {
		Description: "For scrape platform types, the set of urls to scrape.",
		Type:        schema.TypeList,
		Optional:    true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	"scrape_frequency_secs": {
		Description: "For scrape platform types, how often to scrape the URLs.",
		Type:        schema.TypeInt,
		Optional:    true,
	},
	"scrape_request_headers": {
		Description: "An array of request headers, each containing `name` and `value` fields.",
		Type:        schema.TypeList,
		Optional:    true,
		Elem: &schema.Schema{
			Type: schema.TypeMap,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	},
	"scrape_request_basic_auth_user": {
		Description: "Basic auth username for scraping.",
		Type:        schema.TypeString,
		Optional:    true,
	},
	"scrape_request_basic_auth_password": {
		Description: "Basic auth password for scraping.",
		Type:        schema.TypeString,
		Optional:    true,
		Sensitive:   true,
	},
	"data_region": {
		Description: "Region where we store your data.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
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
		CustomizeDiff: validateSource,
		Description:   "This resource allows you to create, modify, and delete your Sources. For more information about the Sources API check https://betterstack.com/docs/logs/api/list-all-existing-sources/",
		Schema:        sourceSchema,
	}
}

type source struct {
	Name                           *string                   `json:"name,omitempty"`
	Token                          *string                   `json:"token,omitempty"`
	TableName                      *string                   `json:"table_name,omitempty"`
	Platform                       *string                   `json:"platform,omitempty"`
	IngestingHost                  *string                   `json:"ingesting_host,omitempty"`
	IngestingPaused                *bool                     `json:"ingesting_paused,omitempty"`
	LogsRetention                  *int                      `json:"logs_retention,omitempty"`
	MetricsRetention               *int                      `json:"metrics_retention,omitempty"`
	LiveTailPattern                *string                   `json:"live_tail_pattern,omitempty"`
	CreatedAt                      *string                   `json:"created_at,omitempty"`
	UpdatedAt                      *string                   `json:"updated_at,omitempty"`
	TeamName                       *string                   `json:"team_name,omitempty"`
	ScrapeURLs                     *[]string                 `json:"scrape_urls,omitempty"`
	ScrapeFrequencySecs            *int                      `json:"scrape_frequency_secs,omitempty"`
	ScrapeRequestHeaders           *[]map[string]interface{} `json:"scrape_request_headers,omitempty"`
	ScrapeRequestBasicAuthUser     *string                   `json:"scrape_request_basic_auth_user,omitempty"`
	ScrapeRequestBasicAuthPassword *string                   `json:"scrape_request_basic_auth_password,omitempty"`
	DataRegion                     *string                   `json:"data_region,omitempty"`
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
		{k: "ingesting_host", v: &in.IngestingHost},
		{k: "ingesting_paused", v: &in.IngestingPaused},
		{k: "logs_retention", v: &in.LogsRetention},
		{k: "metrics_retention", v: &in.MetricsRetention},
		{k: "live_tail_pattern", v: &in.LiveTailPattern},
		{k: "created_at", v: &in.CreatedAt},
		{k: "updated_at", v: &in.UpdatedAt},
		{k: "scrape_urls", v: &in.ScrapeURLs},
		{k: "scrape_frequency_secs", v: &in.ScrapeFrequencySecs},
		{k: "scrape_request_headers", v: &in.ScrapeRequestHeaders},
		{k: "scrape_request_basic_auth_user", v: &in.ScrapeRequestBasicAuthUser},
		{k: "scrape_request_basic_auth_password", v: &in.ScrapeRequestBasicAuthPassword},
		{k: "data_region", v: &in.DataRegion},
	}
}

func sourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in source
	for _, e := range sourceRef(&in) {
		load(d, e.k, e.v)
	}

	load(d, "team_name", &in.TeamName)

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

func validateSource(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	if diff.Id() != "" && diff.HasChange("data_region") {
		return fmt.Errorf("data_region cannot be changed after source is created")
	}

	if err := validateRequestHeaders(ctx, diff, v); err != nil {
		return err
	}

	return nil
}

func validateRequestHeaders(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	if headers, ok := diff.GetOk("scrape_request_headers"); ok {
		for _, header := range headers.([]interface{}) {
			headerMap := header.(map[string]interface{})
			if err := validateRequestHeader(headerMap); err != nil {
				return fmt.Errorf("Invalid request header %v: %v", headerMap, err)
			}
		}
	}
	return nil
}

func validateRequestHeader(header map[string]interface{}) error {
	if len(header) == 0 {
		// Headers with calculated fields that are not known at the time will be passed as empty maps, ignore them
		return nil
	}

	name, nameOk := header["name"].(string)
	value, valueOk := header["value"].(string)

	if !nameOk || name == "" {
		return fmt.Errorf("must contain 'name' key with a non-empty string value")
	}

	if !valueOk || value == "" {
		return fmt.Errorf("must contain 'value' key with a non-empty string value")
	}

	if len(header) != 2 {
		return fmt.Errorf("must only contain 'name' and 'value' keys")
	}

	return nil
}
