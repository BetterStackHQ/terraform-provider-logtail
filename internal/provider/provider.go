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
			"api_retry_max": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     4,
				Description: "Maximum number of retries for API requests.",
			},
			"api_retry_wait_min": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     10,
				Description: "Minimum time to wait between retries in seconds.",
			},
			"api_retry_wait_max": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     300,
				Description: "Maximum time to wait between retries in seconds.",
			},
			"api_timeout": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     60,
				Description: "Timeout for individual HTTP requests in seconds.",
			},
			"api_rate_limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     8,
				Description: "Maximum number of API requests per second. 0 means no limit.",
			},
			"api_rate_burst": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Burst size for rate limiter, allows temporary bursts above the rate limit. 0 means use automatic default (2x rate limit, minimum 10).",
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
			"logtail_connection":               newConnectionDataSource(),
			"logtail_dashboard":                newDashboardDataSource(),
			"logtail_dashboard_template":       newDashboardTemplateDataSource(),
			"logtail_collector":                newCollectorDataSource(),
			"logtail_exploration_group":        newExplorationGroupDataSource(),
			"logtail_exploration":              newExplorationDataSource(),
			"logtail_exploration_alert":        newExplorationAlertDataSource(),
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
			"logtail_connection":               newConnectionResource(),
			"logtail_dashboard":                newDashboardResource(),
			"logtail_collector":                newCollectorResource(),
			"logtail_exploration_group":        newExplorationGroupResource(),
			"logtail_exploration":              newExplorationResource(),
			"logtail_exploration_alert":        newExplorationAlertResource(),
		},
		ConfigureContextFunc: func(ctx context.Context, r *schema.ResourceData) (interface{}, diag.Diagnostics) {
			var userAgent string
			if spec.version != "" {
				userAgent = "terraform-provider-logtail/" + spec.version
			}

			timeout := time.Duration(r.Get("api_timeout").(int)) * time.Second

			c, err := newClient(ClientConfig{
				BaseURL:      spec.url,
				Token:        r.Get("api_token").(string),
				UserAgent:    userAgent,
				HTTPClient:   &http.Client{Timeout: timeout},
				RetryMax:     r.Get("api_retry_max").(int),
				RetryWaitMin: time.Duration(r.Get("api_retry_wait_min").(int)) * time.Second,
				RetryWaitMax: time.Duration(r.Get("api_retry_wait_max").(int)) * time.Second,
				RateLimit:    r.Get("api_rate_limit").(int),
				RateBurst:    r.Get("api_rate_burst").(int),
			})
			return c, diag.FromErr(err)
		},
	}
}
