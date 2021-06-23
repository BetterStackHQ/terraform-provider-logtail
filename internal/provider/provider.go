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
		url: "https://logtail.com",
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
				Description: "Logtail API Token. The value can be omitted if `LOGTAIL_API_TOKEN` environment variable is set. See https://docs.logtail.com/api/getting-started#obtaining-an-api-token on how to obtain the API token for your team.",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"logtail_source": newSourceDataSource(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"logtail_source": newSourceResource(),
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
