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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var metricSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this metric.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"source_id": {
		Description: "The ID of the source this metric belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"name": {
		Description: "The name of this metric.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"sql_expression": {
		Description: "The SQL expression used to extract the metric value.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"aggregations": {
		Description: "The list of aggregations to perform on the metric.",
		Type:        schema.TypeList,
		Required:    true,
		ForceNew:    true,
		Elem:        &schema.Schema{Type: schema.TypeString, ValidateFunc: validation.StringInSlice([]string{"avg", "count", "uniq", "max", "min", "anyLast", "sum", "p50", "p90", "p95", "p99"}, false)},
	},
	"type": {
		Description:  "The type of the metric.",
		Type:         schema.TypeString,
		Required:     true,
		ForceNew:     true,
		ValidateFunc: validation.StringInSlice([]string{"string_low_cardinality", "int64_delta", "float64_delta", "datetime64_delta", "boolean"}, false),
	},
}

func newMetricResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: metricCreate,
		ReadContext:   metricLookup,
		DeleteContext: metricDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Create custom metrics from your log data using SQL expressions. Transform log events into quantifiable metrics for monitoring, alerting, and dashboard visualization across your infrastructure and applications. [Learn more](https://betterstack.com/docs/logs/dashboards/logs-to-metrics/).",
		Schema:      metricSchema,
	}
}

type metric struct {
	SourceID      *string   `json:"source_id,omitempty"`
	Name          *string   `json:"name,omitempty"`
	SQLExpression *string   `json:"sql_expression,omitempty"`
	Aggregations  *[]string `json:"aggregations,omitempty"`
	Type          *string   `json:"type,omitempty"`
}

type metricHTTPResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes metric `json:"attributes"`
	} `json:"data"`
}

type metricPageHTTPResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes metric `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func metricRef(in *metric) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "sql_expression", v: &in.SQLExpression},
		{k: "aggregations", v: &in.Aggregations},
		{k: "type", v: &in.Type},
	}
}

func metricCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in metric
	for _, e := range metricRef(&in) {
		load(d, e.k, e.v)
	}
	sourceId := d.Get("source_id").(string)
	var out metricHTTPResponse
	if err := resourceCreate(ctx, meta, fmt.Sprintf("/api/v2/sources/%s/metrics", url.PathEscape(sourceId)), &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return metricCopyAttrs(d, &out.Data.Attributes)
}

func metricLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	sourceId := d.Get("source_id").(string)
	fetch := func(page int) (*metricPageHTTPResponse, error) {
		res, err := meta.(*client).Get(ctx, fmt.Sprintf("/api/v2/sources/%s/metrics?page=%d", url.PathEscape(sourceId), page))
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
		var tr metricPageHTTPResponse
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
				if derr := metricCopyAttrs(d, &e.Attributes); derr != nil {
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

func metricCopyAttrs(d *schema.ResourceData, in *metric) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range metricRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	return derr
}

func metricDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceId := d.Get("source_id").(string)
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/sources/%s/metrics/%s", url.PathEscape(sourceId), url.PathEscape(d.Id())))
}
