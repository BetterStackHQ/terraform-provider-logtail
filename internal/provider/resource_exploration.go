package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var explorationSchema = map[string]*schema.Schema{
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
		Description: "The ID of this exploration.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this exploration.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"date_range_from": {
		Description: "The start of the date range (e.g., 'now-3h', 'now-24h').",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"date_range_to": {
		Description: "The end of the date range (e.g., 'now').",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"exploration_group_id": {
		Description: "The ID of the exploration group this exploration belongs to. Use 0 to remove from group.",
		Type:        schema.TypeInt,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Check if the attribute is actually set in config using raw config
			rawConfig := d.GetRawConfig()
			if !rawConfig.IsNull() && rawConfig.IsKnown() {
				val := rawConfig.GetAttr("exploration_group_id")
				if val.IsNull() || !val.IsKnown() {
					// null/unset in config means "don't manage" - suppress diff
					return true
				}
			}
			// 0 in config means "explicitly no group" - suppress only if state is also 0 or empty
			if new == "0" {
				return old == "0" || old == ""
			}
			return false
		},
	},
	"created_at": {
		Description: "The time when this exploration was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this exploration was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"chart": {
		Description: "The chart configuration for this exploration.",
		Type:        schema.TypeList,
		Required:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"chart_type": {
					Description: "The type of chart (e.g., 'line_chart', 'bar_chart', 'pie_chart', 'number_chart', 'table_chart', 'tail_chart', 'static_text_chart').",
					Type:        schema.TypeString,
					Required:    true,
				},
				"name": {
					Description: "The name of the chart. Automatically set to the exploration name by the API.",
					Type:        schema.TypeString,
					Computed:    true,
				},
				"description": {
					Description: "The description of the chart.",
					Type:        schema.TypeString,
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
			},
		},
	},
	"query": {
		Description: "The queries for this exploration. At least one query is required.",
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
				},
				"query_type": {
					Description: "The type of query: 'sql_expression', 'tail_query', or 'static_text'. Note: 'pql_expression', 'query_builder', and 'funnel_query' are read-only.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"sql_query": {
					Description: "The SQL query string. Required when query_type is 'sql_expression'.",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"where_condition": {
					Description: "The WHERE condition for filtering. Required when query_type is 'tail_query'.",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"static_text": {
					Description: "The static text content (markdown). Required when query_type is 'static_text'.",
					Type:        schema.TypeString,
					Optional:    true,
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
	"variable": {
		Description: "Variables for this exploration. Default variables (time, start_time, end_time, source) are auto-created.",
		Type:        schema.TypeList,
		Optional:    true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Description: "The name of the variable (used as {{name}} in queries).",
					Type:        schema.TypeString,
					Required:    true,
				},
				"variable_type": {
					Description: "The type of variable: 'source', 'string', 'number', 'date', 'datetime', 'boolean', 'sql_expression', 'select_value', or 'select_with_sql'.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"values": {
					Description: "Predefined values for 'select_value' type variables.",
					Type:        schema.TypeList,
					Optional:    true,
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
				"default_values": {
					Description: "Default selected values for the variable.",
					Type:        schema.TypeList,
					Optional:    true,
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
				"sql_definition": {
					Description: "SQL definition for 'sql_expression' or 'select_with_sql' type variables.",
					Type:        schema.TypeString,
					Optional:    true,
				},
			},
		},
	},
}

func newExplorationResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: explorationCreate,
		ReadContext:   explorationRead,
		UpdateContext: explorationUpdate,
		DeleteContext: explorationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "This resource allows you to create, modify, and delete Explorations in Better Stack Telemetry. Explorations are interactive charts with queries and variables.",
		Schema:      explorationSchema,
	}
}

// API structs
type explorationChart struct {
	ChartType   *string                `json:"chart_type,omitempty"`
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

type explorationQuery struct {
	ID             *int    `json:"id,omitempty"`
	Name           *string `json:"name,omitempty"`
	QueryType      *string `json:"query_type,omitempty"`
	SQLQuery       *string `json:"sql_query,omitempty"`
	WhereCondition *string `json:"where_condition,omitempty"`
	StaticText     *string `json:"static_text,omitempty"`
	SourceVariable *string `json:"source_variable,omitempty"`
}

type explorationVariable struct {
	Name          *string  `json:"name,omitempty"`
	VariableType  *string  `json:"variable_type,omitempty"`
	Values        []string `json:"values,omitempty"`
	DefaultValues []string `json:"default_values,omitempty"`
	SQLDefinition *string  `json:"sql_definition,omitempty"`
}

type exploration struct {
	Name               *string               `json:"name,omitempty"`
	DateRangeFrom      *string               `json:"date_range_from,omitempty"`
	DateRangeTo        *string               `json:"date_range_to,omitempty"`
	ExplorationGroupID *int                  `json:"exploration_group_id,omitempty"`
	Chart              *explorationChart     `json:"chart,omitempty"`
	Queries            []explorationQuery    `json:"queries,omitempty"`
	Variables          []explorationVariable `json:"variables,omitempty"`
	CreatedAt          *string               `json:"created_at,omitempty"`
	UpdatedAt          *string               `json:"updated_at,omitempty"`
	TeamName           *string               `json:"team_name,omitempty"`
}

type explorationHTTPResponse struct {
	Data struct {
		ID         string      `json:"id"`
		Attributes exploration `json:"attributes"`
	} `json:"data"`
}

func explorationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := loadExploration(d)

	var out explorationHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v2/explorations", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return explorationCopyAttrs(d, &out.Data.Attributes)
}

func explorationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out explorationHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/explorations/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}
	return explorationCopyAttrs(d, &out.Data.Attributes)
}

func explorationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := loadExploration(d)
	// Clear team_name on update as it's only used for creation
	in.TeamName = nil

	if diags := resourceUpdate(ctx, meta, fmt.Sprintf("/api/v2/explorations/%s", url.PathEscape(d.Id())), &in); diags != nil {
		return diags
	}
	// Read back the resource to get computed values
	return explorationRead(ctx, d, meta)
}

func explorationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/explorations/%s", url.PathEscape(d.Id())))
}

func loadExploration(d *schema.ResourceData) exploration {
	var in exploration

	// Load simple fields
	load(d, "name", &in.Name)
	load(d, "date_range_from", &in.DateRangeFrom)
	load(d, "date_range_to", &in.DateRangeTo)
	load(d, "team_name", &in.TeamName)

	// Load exploration_group_id using intFromResourceData to properly distinguish:
	// - null/unset -> don't send to API (don't care about group)
	// - 0 -> send 0 to API (explicitly remove from group)
	// - N -> send N to API (assign to group N)
	in.ExplorationGroupID = intFromResourceData(d, "exploration_group_id")

	// Load chart
	// Note: chart.name is not accepted by the API on create/update - it's auto-derived from exploration name
	if chartData, ok := d.GetOk("chart"); ok {
		chartList := chartData.([]interface{})
		if len(chartList) > 0 {
			chartMap := chartList[0].(map[string]interface{})
			chart := &explorationChart{}

			if v, ok := chartMap["chart_type"].(string); ok && v != "" {
				chart.ChartType = &v
			}
			// chart.name is read-only - auto-derived from exploration name
			if v, ok := chartMap["description"].(string); ok && v != "" {
				chart.Description = &v
			}
			if v, ok := chartMap["settings"].(string); ok && v != "" && v != "{}" {
				var settings map[string]interface{}
				if err := json.Unmarshal([]byte(v), &settings); err == nil && len(settings) > 0 {
					chart.Settings = settings
				}
			}

			in.Chart = chart
		}
	}

	// Load queries
	if queryData, ok := d.GetOk("query"); ok {
		queryList := queryData.([]interface{})
		queries := make([]explorationQuery, 0, len(queryList))

		for _, qData := range queryList {
			qMap := qData.(map[string]interface{})
			query := explorationQuery{}

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

	// Load variables
	if varData, ok := d.GetOk("variable"); ok {
		varList := varData.([]interface{})
		variables := make([]explorationVariable, 0, len(varList))

		for _, vData := range varList {
			vMap := vData.(map[string]interface{})
			variable := explorationVariable{}

			if v, ok := vMap["name"].(string); ok && v != "" {
				variable.Name = &v
			}
			if v, ok := vMap["variable_type"].(string); ok && v != "" {
				variable.VariableType = &v
			}
			if v, ok := vMap["sql_definition"].(string); ok && v != "" {
				variable.SQLDefinition = &v
			}

			// Load values array
			if valuesData, ok := vMap["values"].([]interface{}); ok && len(valuesData) > 0 {
				values := make([]string, 0, len(valuesData))
				for _, val := range valuesData {
					if s, ok := val.(string); ok {
						values = append(values, s)
					}
				}
				variable.Values = values
			}

			// Load default_values array
			if defaultData, ok := vMap["default_values"].([]interface{}); ok && len(defaultData) > 0 {
				defaults := make([]string, 0, len(defaultData))
				for _, val := range defaultData {
					if s, ok := val.(string); ok {
						defaults = append(defaults, s)
					}
				}
				variable.DefaultValues = defaults
			}

			variables = append(variables, variable)
		}

		in.Variables = variables
	}

	return in
}

func explorationCopyAttrs(d *schema.ResourceData, in *exploration) diag.Diagnostics {
	var derr diag.Diagnostics

	// Copy simple fields
	if in.Name != nil {
		if err := d.Set("name", *in.Name); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.DateRangeFrom != nil {
		if err := d.Set("date_range_from", *in.DateRangeFrom); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.DateRangeTo != nil {
		if err := d.Set("date_range_to", *in.DateRangeTo); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.ExplorationGroupID != nil {
		if err := d.Set("exploration_group_id", *in.ExplorationGroupID); err != nil {
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

	// Copy chart
	if in.Chart != nil {
		chartData := make(map[string]interface{})
		if in.Chart.ChartType != nil {
			chartData["chart_type"] = *in.Chart.ChartType
		}
		// Always set name (Computed field) - use empty string if nil
		if in.Chart.Name != nil {
			chartData["name"] = *in.Chart.Name
		} else {
			chartData["name"] = ""
		}
		// Always set description (Computed field) - use empty string if nil
		if in.Chart.Description != nil {
			chartData["description"] = *in.Chart.Description
		} else {
			chartData["description"] = ""
		}
		// Always set settings (Computed field) - use "{}" if nil or empty
		if len(in.Chart.Settings) > 0 {
			settingsJSON, err := json.Marshal(in.Chart.Settings)
			if err == nil {
				chartData["settings"] = string(settingsJSON)
			} else {
				chartData["settings"] = "{}"
			}
		} else {
			chartData["settings"] = "{}"
		}
		if err := d.Set("chart", []interface{}{chartData}); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy queries
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

	// Copy variables - preserve user's order and only include user-configured variables
	if in.Variables != nil {
		// System variables that are always auto-created and cannot be customized
		systemVars := map[string]bool{
			"time":       true,
			"start_time": true,
			"end_time":   true,
		}

		// Build a map of API-returned variables by name for quick lookup
		apiVarsByName := make(map[string]*explorationVariable)
		for i := range in.Variables {
			if in.Variables[i].Name != nil {
				apiVarsByName[*in.Variables[i].Name] = &in.Variables[i]
			}
		}

		// Get user-configured variables in their original order
		var userVarNames []string
		if varConfig, ok := d.GetOk("variable"); ok {
			for _, v := range varConfig.([]interface{}) {
				vMap := v.(map[string]interface{})
				if name, ok := vMap["name"].(string); ok {
					userVarNames = append(userVarNames, name)
				}
			}
		}

		// Build result in user's order, using API values
		varData := make([]interface{}, 0, len(userVarNames))
		for _, name := range userVarNames {
			// Skip system variables
			if systemVars[name] {
				continue
			}

			apiVar, exists := apiVarsByName[name]
			if !exists {
				continue
			}

			vMap := make(map[string]interface{})
			vMap["name"] = name
			if apiVar.VariableType != nil {
				vMap["variable_type"] = *apiVar.VariableType
			}
			if apiVar.SQLDefinition != nil {
				vMap["sql_definition"] = *apiVar.SQLDefinition
			}
			if apiVar.Values != nil {
				vMap["values"] = apiVar.Values
			}
			if apiVar.DefaultValues != nil {
				vMap["default_values"] = apiVar.DefaultValues
			}
			varData = append(varData, vMap)
		}
		if err := d.Set("variable", varData); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}
