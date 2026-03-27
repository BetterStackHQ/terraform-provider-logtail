package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var dashboardChartSchema = map[string]*schema.Schema{
	"dashboard_id": {
		Description: "The ID of the dashboard this chart belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"id": {
		Description: "The ID of this chart.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"chart_type": {
		Description:  "The type of chart: 'line_chart', 'bar_chart', 'pie_chart', 'number_chart', 'table_chart', 'tail_chart', 'static_text_chart', 'scatter_chart', 'gauge_chart', 'heatmap_chart', 'map_chart', 'text_chart', 'funnel_chart', or 'anomalies_chart'.",
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringInSlice([]string{"line_chart", "bar_chart", "pie_chart", "number_chart", "table_chart", "tail_chart", "static_text_chart", "scatter_chart", "gauge_chart", "heatmap_chart", "map_chart", "text_chart", "funnel_chart", "anomalies_chart"}, false),
	},
	"name": {
		Description: "The name of this chart.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"description": {
		Description: "The description of this chart.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"x": {
		Description: "The horizontal position of this chart in the dashboard grid (0-11).",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"y": {
		Description: "The vertical position of this chart in the dashboard grid.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"w": {
		Description: "The width of this chart in grid units (1-12).",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"h": {
		Description: "The height of this chart in grid units.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"settings": {
		Description: "Chart settings as a JSON string. Settings vary by chart type and include options like unit, decimal_places, legend, stacking, etc.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Treat empty or "{}" as equivalent to not set
			if (old == "" || old == "{}") && (new == "" || new == "{}") {
				return true
			}
			if old == "" || new == "" {
				return false
			}
			// Normalize JSON to compare
			var oldJSON, newJSON interface{}
			if err := json.Unmarshal([]byte(old), &oldJSON); err != nil {
				return false
			}
			if err := json.Unmarshal([]byte(new), &newJSON); err != nil {
				return false
			}
			oldNorm, _ := json.Marshal(oldJSON)
			newNorm, _ := json.Marshal(newJSON)
			return string(oldNorm) == string(newNorm)
		},
	},
	"query": {
		Description: "The queries for this chart. At least one query is required.",
		Type:        schema.TypeList,
		Required:    true,
		MinItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"id": {
					Description: "The ID of this query (read-only).",
					Type:        schema.TypeInt,
					Computed:    true,
				},
				"name": {
					Description: "The name of the query.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"query_type": {
					Description:  "The type of query: 'sql_expression', 'tail_query', or 'static_text'. Note: 'pql_expression', 'query_builder', and 'funnel_query' are read-only.",
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.StringInSlice([]string{"sql_expression", "tail_query", "static_text", "pql_expression", "query_builder", "funnel_query"}, false),
				},
				"sql_query": {
					Description: "The SQL query string. Required when query_type is 'sql_expression'.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"where_condition": {
					Description: "The WHERE condition for filtering. Required when query_type is 'tail_query'.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"static_text": {
					Description: "The static text content (markdown). Required when query_type is 'static_text'.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
				"source_variable": {
					Description: "The source variable reference (default: 'source').",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
			},
		},
	},
	"created_at": {
		Description: "The time when this chart was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this chart was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newDashboardChartResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: dashboardChartCreate,
		ReadContext:   dashboardChartRead,
		UpdateContext: dashboardChartUpdate,
		DeleteContext: dashboardChartDelete,
		Importer: &schema.ResourceImporter{
			StateContext: dashboardChartImportState,
		},
		Description: "This resource allows you to create, modify, and delete Charts in a Dashboard in Better Stack Telemetry.",
		Schema:      dashboardChartSchema,
	}
}

type dashboardChartQuery struct {
	ID             *int    `json:"id,omitempty"`
	Name           *string `json:"name,omitempty"`
	QueryType      *string `json:"query_type,omitempty"`
	SQLQuery       *string `json:"sql_query,omitempty"`
	WhereCondition *string `json:"where_condition,omitempty"`
	StaticText     *string `json:"static_text,omitempty"`
	SourceVariable *string `json:"source_variable,omitempty"`
}

type dashboardChart struct {
	ChartType   *string                `json:"chart_type,omitempty"`
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	X           *int                   `json:"x,omitempty"`
	Y           *int                   `json:"y,omitempty"`
	W           *int                   `json:"w,omitempty"`
	H           *int                   `json:"h,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
	Queries     []dashboardChartQuery  `json:"queries,omitempty"`
	CreatedAt   *string                `json:"created_at,omitempty"`
	UpdatedAt   *string                `json:"updated_at,omitempty"`
}

type dashboardChartHTTPResponse struct {
	Data struct {
		ID         string         `json:"id"`
		Attributes dashboardChart `json:"attributes"`
	} `json:"data"`
}

func dashboardChartCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID := d.Get("dashboard_id").(string)
	in := loadDashboardChart(d)

	var out dashboardChartHTTPResponse
	if err := resourceCreate(ctx, meta, fmt.Sprintf("/api/v2/dashboards/%s/charts", url.PathEscape(dashboardID)), &in, &out); err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%s/%s", dashboardID, out.Data.ID))
	return dashboardChartCopyAttrs(d, &out.Data.Attributes)
}

func dashboardChartRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, err := parseDashboardChartID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var out dashboardChartHTTPResponse
	if diags, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(),
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s", url.PathEscape(dashboardID), url.PathEscape(chartID)), &out); diags != nil {
		return diags
	} else if !ok {
		d.SetId("")
		return nil
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return diag.FromErr(err)
	}

	return dashboardChartCopyAttrs(d, &out.Data.Attributes)
}

func dashboardChartUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, err := parseDashboardChartID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	in := loadDashboardChart(d)

	if diags := resourceUpdate(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s", url.PathEscape(dashboardID), url.PathEscape(chartID)), &in); diags != nil {
		return diags
	}
	return dashboardChartRead(ctx, d, meta)
}

func dashboardChartDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, err := parseDashboardChartID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceDelete(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s", url.PathEscape(dashboardID), url.PathEscape(chartID)))
}

func dashboardChartImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	dashboardID, chartID, err := parseDashboardChartID(d.Id())
	if err != nil {
		return nil, err
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return nil, err
	}

	d.SetId(fmt.Sprintf("%s/%s", dashboardID, chartID))
	return []*schema.ResourceData{d}, nil
}

func parseDashboardChartID(id string) (dashboardID, chartID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid dashboard chart ID format %q, expected 'dashboard_id/chart_id'", id)
	}
	return parts[0], parts[1], nil
}

func loadDashboardChart(d *schema.ResourceData) dashboardChart {
	var in dashboardChart

	load(d, "chart_type", &in.ChartType)
	load(d, "name", &in.Name)
	load(d, "description", &in.Description)
	in.X = intFromResourceData(d, "x")
	in.Y = intFromResourceData(d, "y")
	in.W = intFromResourceData(d, "w")
	in.H = intFromResourceData(d, "h")

	// Load settings
	if v, ok := d.GetOk("settings"); ok {
		s := v.(string)
		if s != "" && s != "{}" {
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(s), &settings); err == nil && len(settings) > 0 {
				in.Settings = settings
			}
		}
	}

	// Load queries
	if queryData, ok := d.GetOk("query"); ok {
		queryList := queryData.([]interface{})
		queries := make([]dashboardChartQuery, 0, len(queryList))

		for _, qData := range queryList {
			qMap := qData.(map[string]interface{})
			query := dashboardChartQuery{}

			if v, ok := qMap["name"].(string); ok && v != "" {
				query.Name = &v
			}
			if v, ok := qMap["query_type"].(string); ok && v != "" {
				query.QueryType = &v
			}
			if v, ok := qMap["sql_query"].(string); ok && v != "" {
				query.SQLQuery = &v
			}
			if v, ok := qMap["where_condition"].(string); ok && v != "" {
				query.WhereCondition = &v
			}
			if v, ok := qMap["static_text"].(string); ok && v != "" {
				query.StaticText = &v
			}
			if v, ok := qMap["source_variable"].(string); ok && v != "" {
				query.SourceVariable = &v
			}

			queries = append(queries, query)
		}

		in.Queries = queries
	}

	return in
}

func dashboardChartCopyAttrs(d *schema.ResourceData, in *dashboardChart) diag.Diagnostics {
	var derr diag.Diagnostics

	if in.ChartType != nil {
		if err := d.Set("chart_type", *in.ChartType); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Name != nil {
		if err := d.Set("name", *in.Name); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Description != nil {
		if err := d.Set("description", *in.Description); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	} else {
		if err := d.Set("description", ""); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.X != nil {
		if err := d.Set("x", *in.X); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Y != nil {
		if err := d.Set("y", *in.Y); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.W != nil {
		if err := d.Set("w", *in.W); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.H != nil {
		if err := d.Set("h", *in.H); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Settings
	if len(in.Settings) > 0 {
		settingsJSON, err := json.Marshal(in.Settings)
		if err == nil {
			if err := d.Set("settings", string(settingsJSON)); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		} else {
			if err := d.Set("settings", "{}"); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		}
	} else {
		if err := d.Set("settings", "{}"); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Queries
	if in.Queries != nil {
		queryData := make([]interface{}, 0, len(in.Queries))
		for _, q := range in.Queries {
			qMap := make(map[string]interface{})
			if q.ID != nil {
				qMap["id"] = *q.ID
			}
			if q.Name != nil {
				qMap["name"] = *q.Name
			}
			if q.QueryType != nil {
				qMap["query_type"] = *q.QueryType
			}
			if q.SQLQuery != nil {
				qMap["sql_query"] = *q.SQLQuery
			}
			if q.WhereCondition != nil {
				qMap["where_condition"] = *q.WhereCondition
			}
			if q.StaticText != nil {
				qMap["static_text"] = *q.StaticText
			}
			if q.SourceVariable != nil {
				qMap["source_variable"] = *q.SourceVariable
			}
			queryData = append(queryData, qMap)
		}
		if err := d.Set("query", queryData); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	if in.CreatedAt != nil {
		if err := d.Set("created_at", *in.CreatedAt); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.UpdatedAt != nil {
		if err := d.Set("updated_at", *in.UpdatedAt); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}
