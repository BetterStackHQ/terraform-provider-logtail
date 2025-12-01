package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var timeSeriesTypes = []string{
	"string", "string_low_cardinality", "int64_delta", "int64", "uint64_delta", "uint64",
	"float64_delta", "datetime64_delta", "boolean", "array_bfloat16", "array_float32",
}

var warehouseTimeSeriesSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this time series.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"source_id": {
		Description: "The ID of the Warehouse source to create the time series for.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"name": {
		Description: "The name of the time series. Must contain only letters, numbers, and underscores.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"type": {
		Description: `The data type of the time series. Valid types are: ` + "`string`, `string_low_cardinality`, `int64_delta`, `int64`, `uint64_delta`, `uint64`, `float64_delta`, `datetime64_delta`, `boolean`, `array_bfloat16`, `array_float32`",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
		ValidateDiagFunc: func(v interface{}, path cty.Path) diag.Diagnostics {
			s := v.(string)
			for _, tsType := range timeSeriesTypes {
				if s == tsType {
					return nil
				}
			}
			return diag.Diagnostics{
				diag.Diagnostic{
					AttributePath: path,
					Severity:      diag.Error,
					Summary:       `Invalid "type"`,
					Detail:        fmt.Sprintf("Expected one of %v", timeSeriesTypes),
				},
			}
		},
	},
	"sql_expression": {
		Description: "The SQL expression used to compute the time series. For example `JSONExtract(raw, 'response_time', 'Nullable(Float64)')`.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"aggregations": {
		Description: "An array of aggregation functions (e.g., `avg`, `min`, `max`). If omitted, no aggregations are applied.",
		Type:        schema.TypeList,
		Optional:    true,
		ForceNew:    true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	},
	"expression_index": {
		Description: "The type of vector index to apply (e.g., `vector_similarity`). Only applicable for vector types (`array_bfloat16`, `array_float32`).",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
	},
	"vector_dimension": {
		Description:  "The vector dimension if `expression_index` is `vector_similarity` (e.g., `512`). Supported values: 256, 384, 512, 768, 1024, 1536, 3072, 4096, 10752.",
		Type:         schema.TypeInt,
		Optional:     true,
		ForceNew:     true,
		ValidateFunc: validation.IntInSlice([]int{256, 384, 512, 768, 1024, 1536, 3072, 4096, 10752}),
	},
	"vector_distance_function": {
		Description:  "The distance function to use for vector similarity (e.g., `cosine`, `l2`).",
		Type:         schema.TypeString,
		Optional:     true,
		ForceNew:     true,
		ValidateFunc: validation.StringInSlice([]string{"cosine", "l2"}, false),
	},
}

func newWarehouseTimeSeriesResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: warehouseTimeSeriesCreate,
		ReadContext:   warehouseTimeSeriesRead,
		DeleteContext: warehouseTimeSeriesDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create and delete your Warehouse time series. For more information about the Warehouse API check https://betterstack.com/docs/warehouse/api/time-series/create/",
		Schema:      warehouseTimeSeriesSchema,
	}
}

type warehouseTimeSeries struct {
	Name                   *string   `json:"name,omitempty"`
	SqlExpression          *string   `json:"sql_expression,omitempty"`
	Aggregations           *[]string `json:"aggregations,omitempty"`
	Type                   *string   `json:"type,omitempty"`
	ExpressionIndex        *string   `json:"expression_index,omitempty"`
	VectorDimension        *int      `json:"vector_dimension,omitempty"`
	VectorDistanceFunction *string   `json:"vector_distance_function,omitempty"`
}

type warehouseTimeSeriesHTTPResponse struct {
	Data struct {
		ID         string              `json:"id"`
		Attributes warehouseTimeSeries `json:"attributes"`
	} `json:"data"`
}

type warehouseTimeSeriesPageHTTPResponse struct {
	Data []struct {
		ID         string              `json:"id"`
		Attributes warehouseTimeSeries `json:"attributes"`
	} `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

func warehouseTimeSeriesRef(in *warehouseTimeSeries) []struct {
	k string
	v interface{}
} {
	return []struct {
		k string
		v interface{}
	}{
		{k: "name", v: &in.Name},
		{k: "sql_expression", v: &in.SqlExpression},
		{k: "aggregations", v: &in.Aggregations},
		{k: "type", v: &in.Type},
		{k: "expression_index", v: &in.ExpressionIndex},
		{k: "vector_dimension", v: &in.VectorDimension},
		{k: "vector_distance_function", v: &in.VectorDistanceFunction},
	}
}

func warehouseTimeSeriesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in warehouseTimeSeries
	for _, e := range warehouseTimeSeriesRef(&in) {
		load(d, e.k, e.v)
	}

	sourceID := d.Get("source_id").(string)

	var out warehouseTimeSeriesHTTPResponse
	if err := resourceCreateWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/time_series", url.PathEscape(sourceID)), &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return warehouseTimeSeriesCopyAttrs(d, &out.Data.Attributes)
}

func warehouseTimeSeriesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	fetch := func(page int) (*warehouseTimeSeriesPageHTTPResponse, error) {
		res, err := meta.(*client).do(ctx, "GET", meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/time_series?page=%d", url.PathEscape(sourceID), page), nil)
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
		var tr warehouseTimeSeriesPageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}
	page := 1
	for {
		res, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range res.Data {
			if e.ID == d.Id() {
				if derr := warehouseTimeSeriesCopyAttrs(d, &e.Attributes); derr != nil {
					return derr
				}
				return nil
			}
		}
		page++
		if res.Pagination.Next == "" {
			break
		}
	}
	d.SetId("") // Not found, force "create".
	return nil
}

func warehouseTimeSeriesCopyAttrs(d *schema.ResourceData, in *warehouseTimeSeries) diag.Diagnostics {
	var derr diag.Diagnostics
	for _, e := range warehouseTimeSeriesRef(in) {
		if err := d.Set(e.k, reflect.Indirect(reflect.ValueOf(e.v)).Interface()); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}

func warehouseTimeSeriesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	return resourceDeleteWithBaseURL(ctx, meta, meta.(*client).WarehouseBaseURL(), fmt.Sprintf("/api/v1/sources/%s/time_series/%s", url.PathEscape(sourceID), url.PathEscape(d.Id())))
}
