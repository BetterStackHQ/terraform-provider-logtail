package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type provider struct {
	url     string
	version string
}

type Option func(*provider)

func WithURL(v string) Option {
	return func(p *provider) {
		p.url = v
	}
}

func WithVersion(v string) Option {
	return func(p *provider) {
		p.version = v
	}
}

func New(opts ...Option) *schema.Provider {
	spec := provider{
		url: "https://telemetry.betterstack.com",
	}
	for _, opt := range opts {
		opt(&spec)
	}
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_token": {
				Type:        schema.TypeString,
				Sensitive:   true,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("LOGTAIL_API_TOKEN", nil),
				Description: "Better Stack Telemetry API token. The value can be omitted if `LOGTAIL_API_TOKEN` environment variable is set. See https://betterstack.com/docs/logs/api/getting-started/#get-an-logs-api-token on how to obtain the API token for your team.",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"logtail_source":                   newSourceDataSource(),
			"logtail_metric":                   newMetricDataSource(),
			"logtail_source_group":             newSourceGroupDataSource(),
			"logtail_errors_application":       newErrorsApplicationDataSource(),
			"logtail_errors_application_group": newErrorsApplicationGroupDataSource(),
			"logtail_warehouse_source":         newWarehouseSourceDataSource(),
			"logtail_warehouse_source_group":   newWarehouseSourceGroupDataSource(),
			"logtail_warehouse_embedding":      newWarehouseEmbeddingDataSource(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"logtail_source":                   newSourceResource(),
			"logtail_metric":                   newMetricResource(),
			"logtail_source_group":             newSourceGroupResource(),
			"logtail_errors_application":       newErrorsApplicationResource(),
			"logtail_errors_application_group": newErrorsApplicationGroupResource(),
			"logtail_warehouse_source":         newWarehouseSourceResource(),
			"logtail_warehouse_source_group":   newWarehouseSourceGroupResource(),
			"logtail_warehouse_time_series":    newWarehouseTimeSeriesResource(),
			"logtail_warehouse_embedding":      newWarehouseEmbeddingResource(),
		},
		ConfigureContextFunc: func(ctx context.Context, r *schema.ResourceData) (interface{}, diag.Diagnostics) {
			var userAgent string
			if spec.version != "" {
				userAgent = "terraform-provider-logtail/" + spec.version
			}
			c, err := newClient(spec.url, r.Get("api_token").(string),
				withHTTPClient(&http.Client{
					Timeout: time.Second * 60,
				}),
				withUserAgent(userAgent))
			return c, diag.FromErr(err)
		},
	}
}
