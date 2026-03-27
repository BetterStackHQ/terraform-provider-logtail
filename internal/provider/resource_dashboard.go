package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var dashboardSchema = map[string]*schema.Schema{
	"id": {
		Description: "The ID of this dashboard.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of the dashboard.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"data": {
		Description: "The dashboard configuration data as a JSON string. When set, the dashboard is created via the import API and any change forces re-creation. Cannot be combined with individual fields like refresh_interval, date_range_from, chart and variable blocks, etc.",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
	},
	"team_name": {
		Description: "The team name to associate with the dashboard when using a global API token.",
		Type:        schema.TypeString,
		Optional:    true,
		Default:     nil,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			return d.Id() != ""
		},
	},
	"team_id": {
		Description: "The team ID of the dashboard.",
		Type:        schema.TypeInt,
		Computed:    true,
	},
	"dashboard_group_id": {
		Description: "The ID of the dashboard group this dashboard belongs to. Use 0 to remove from group.",
		Type:        schema.TypeInt,
		Optional:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			rawConfig := d.GetRawConfig()
			if !rawConfig.IsNull() && rawConfig.IsKnown() {
				val := rawConfig.GetAttr("dashboard_group_id")
				if val.IsNull() || !val.IsKnown() {
					return true
				}
			}
			if new == "0" {
				return old == "0" || old == ""
			}
			return false
		},
	},
	"refresh_interval": {
		Description: "The auto-refresh interval in seconds.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
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
	"source_eligibility_sql": {
		Description: "SQL expression to filter eligible sources.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"variable": {
		Description: "Variables for this dashboard. Default variables (time, start_time, end_time, source) are auto-created.",
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
					Description: "The type of variable: 'source', 'string', 'number', 'date', 'datetime', 'boolean', 'sql_expression', 'select_value', 'select_with_sql', or 'multi_select_with_sql'.",
					Type:        schema.TypeString,
					Required:    true,
				},
				"values": {
					Description: "Predefined values for 'select_value' type variables.",
					Type:        schema.TypeList,
					Optional:    true,
					Computed:    true,
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
				"default_values": {
					Description: "Default selected values for the variable.",
					Type:        schema.TypeList,
					Optional:    true,
					Computed:    true,
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
				"sql_definition": {
					Description: "SQL definition for 'select_with_sql' or 'multi_select_with_sql' type variables.",
					Type:        schema.TypeString,
					Optional:    true,
					Computed:    true,
				},
			},
		},
	},
	"created_at": {
		Description: "The time when this dashboard was created.",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this dashboard was last updated.",
		Type:        schema.TypeString,
		Computed:    true,
	},
}

type dashboardVariable struct {
	Name          *string  `json:"name,omitempty"`
	VariableType  *string  `json:"variable_type,omitempty"`
	Values        []string `json:"values,omitempty"`
	DefaultValues []string `json:"default_values,omitempty"`
	SQLDefinition *string  `json:"sql_definition,omitempty"`
}

type dashboard struct {
	Name                 *string             `json:"name,omitempty"`
	Data                 interface{}         `json:"data,omitempty"`
	TeamName             *string             `json:"team_name,omitempty"`
	TeamId               *int                `json:"team_id,omitempty"`
	DashboardGroupID     *int                `json:"dashboard_group_id,omitempty"`
	RefreshInterval      *int                `json:"refresh_interval,omitempty"`
	DateRangeFrom        *string             `json:"date_range_from,omitempty"`
	DateRangeTo          *string             `json:"date_range_to,omitempty"`
	SourceEligibilitySQL *string             `json:"source_eligibility_sql,omitempty"`
	Variables            []dashboardVariable `json:"variables,omitempty"`
	CreatedAt            *string             `json:"created_at,omitempty"`
	UpdatedAt            *string             `json:"updated_at,omitempty"`
}

type dashboardHTTPResponse struct {
	Data struct {
		ID         string    `json:"id"`
		Type       string    `json:"type"`
		Attributes dashboard `json:"attributes"`
	} `json:"data"`
}

type dashboardExportResponse struct {
	ID   StringOrInt            `json:"id"`
	Name string                 `json:"name"`
	Data map[string]interface{} `json:"data"`
}

func newDashboardResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: dashboardCreate,
		ReadContext:   dashboardRead,
		UpdateContext: dashboardUpdate,
		DeleteContext: dashboardDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: validateDashboard,
		Description:   "This resource allows you to create and manage dashboards. Use 'data' for import mode (JSON blob, re-created on change) or individual fields for CRUD mode (updatable). For more information about the Dashboard API check https://betterstack.com/docs/logs/api/dashboards/",
		Schema:        dashboardSchema,
	}
}

func isDashboardImportMode(d *schema.ResourceData) bool {
	if v, ok := d.GetOk("data"); ok && v.(string) != "" {
		return true
	}
	return false
}

func validateDashboard(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	data := diff.Get("data").(string)
	hasData := data != ""

	crudFields := []string{"refresh_interval", "date_range_from", "date_range_to", "source_eligibility_sql"}
	hasCrudFields := false
	for _, field := range crudFields {
		if v, ok := diff.GetOk(field); ok {
			switch val := v.(type) {
			case string:
				if val != "" {
					hasCrudFields = true
				}
			case int:
				if val != 0 {
					hasCrudFields = true
				}
			}
		}
	}
	// Check variable block
	if v, ok := diff.GetOk("variable"); ok {
		if varList, ok := v.([]interface{}); ok && len(varList) > 0 {
			hasCrudFields = true
		}
	}

	if hasData && hasCrudFields {
		return fmt.Errorf("cannot use 'data' (import mode) together with individual dashboard fields like 'refresh_interval', 'date_range_from', 'variable', etc. Either provide 'data' to import a dashboard from JSON, or use individual fields for CRUD mode")
	}

	if hasData {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
			return fmt.Errorf("The 'data' field must contain valid JSON. Error: %v", err)
		}
	}

	return nil
}

func dashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if isDashboardImportMode(d) {
		return dashboardCreateImportMode(ctx, d, meta)
	}
	return dashboardCreateCRUDMode(ctx, d, meta)
}

func dashboardCreateImportMode(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in dashboard
	load(d, "name", &in.Name)
	load(d, "team_name", &in.TeamName)

	userData := d.Get("data").(string)
	var dataObj interface{}
	if err := json.Unmarshal([]byte(userData), &dataObj); err != nil {
		return diag.FromErr(fmt.Errorf("invalid JSON in 'data' field: %w", err))
	}
	in.Data = dataObj

	var out dashboardHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v2/dashboards/import", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)

	if out.Data.Attributes.TeamId != nil {
		if err := d.Set("team_id", *out.Data.Attributes.TeamId); err != nil {
			return diag.FromErr(err)
		}
	}
	if out.Data.Attributes.CreatedAt != nil {
		if err := d.Set("created_at", *out.Data.Attributes.CreatedAt); err != nil {
			return diag.FromErr(err)
		}
	}
	if out.Data.Attributes.UpdatedAt != nil {
		if err := d.Set("updated_at", *out.Data.Attributes.UpdatedAt); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("data", userData); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func dashboardCreateCRUDMode(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := loadDashboardCRUD(d)

	var out dashboardHTTPResponse
	if err := resourceCreate(ctx, meta, "/api/v2/dashboards", &in, &out); err != nil {
		return err
	}
	d.SetId(out.Data.ID)
	return dashboardCopyCRUDAttrs(d, &out.Data.Attributes)
}

func dashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out dashboardHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("")
		return nil
	}

	if isDashboardImportMode(d) {
		return dashboardReadImportMode(d, &out.Data.Attributes)
	}
	return dashboardCopyCRUDAttrs(d, &out.Data.Attributes)
}

func dashboardReadImportMode(d *schema.ResourceData, attrs *dashboard) diag.Diagnostics {
	if attrs.Name != nil {
		if err := d.Set("name", *attrs.Name); err != nil {
			return diag.FromErr(err)
		}
	}
	if attrs.TeamId != nil {
		if err := d.Set("team_id", *attrs.TeamId); err != nil {
			return diag.FromErr(err)
		}
	}
	if attrs.DashboardGroupID != nil {
		if err := d.Set("dashboard_group_id", *attrs.DashboardGroupID); err != nil {
			return diag.FromErr(err)
		}
	}
	if attrs.CreatedAt != nil {
		if err := d.Set("created_at", attrs.CreatedAt); err != nil {
			return diag.FromErr(err)
		}
	}
	if attrs.UpdatedAt != nil {
		if err := d.Set("updated_at", attrs.UpdatedAt); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func dashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in dashboard

	if isDashboardImportMode(d) {
		// Import mode: only name and dashboard_group_id can be updated
		load(d, "name", &in.Name)
		in.DashboardGroupID = intFromResourceData(d, "dashboard_group_id")
	} else {
		in = loadDashboardCRUD(d)
	}

	// Clear team_name on update as it's only used for creation
	in.TeamName = nil

	if diags := resourceUpdate(ctx, meta, fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(d.Id())), &in); diags != nil {
		return diags
	}
	return dashboardRead(ctx, d, meta)
}

func dashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(d.Id())))
}

func loadDashboardCRUD(d *schema.ResourceData) dashboard {
	var in dashboard
	load(d, "name", &in.Name)
	load(d, "team_name", &in.TeamName)
	load(d, "date_range_from", &in.DateRangeFrom)
	load(d, "date_range_to", &in.DateRangeTo)
	load(d, "source_eligibility_sql", &in.SourceEligibilitySQL)
	in.DashboardGroupID = intFromResourceData(d, "dashboard_group_id")
	in.RefreshInterval = intFromResourceData(d, "refresh_interval")

	// Load variables
	if varData, ok := d.GetOk("variable"); ok {
		varList := varData.([]interface{})
		variables := make([]dashboardVariable, 0, len(varList))

		for _, vData := range varList {
			vMap := vData.(map[string]interface{})
			variable := dashboardVariable{}

			if v, ok := vMap["name"].(string); ok && v != "" {
				variable.Name = &v
			}
			if v, ok := vMap["variable_type"].(string); ok && v != "" {
				variable.VariableType = &v
			}
			if v, ok := vMap["sql_definition"].(string); ok && v != "" {
				variable.SQLDefinition = &v
			}
			if valuesData, ok := vMap["values"].([]interface{}); ok && len(valuesData) > 0 {
				values := make([]string, 0, len(valuesData))
				for _, val := range valuesData {
					if s, ok := val.(string); ok {
						values = append(values, s)
					}
				}
				variable.Values = values
			}
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

func dashboardCopyCRUDAttrs(d *schema.ResourceData, in *dashboard) diag.Diagnostics {
	var derr diag.Diagnostics

	if in.Name != nil {
		if err := d.Set("name", *in.Name); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.TeamId != nil {
		if err := d.Set("team_id", *in.TeamId); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.DashboardGroupID != nil {
		if err := d.Set("dashboard_group_id", *in.DashboardGroupID); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.RefreshInterval != nil {
		if err := d.Set("refresh_interval", *in.RefreshInterval); err != nil {
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
	if in.SourceEligibilitySQL != nil {
		if err := d.Set("source_eligibility_sql", *in.SourceEligibilitySQL); err != nil {
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

	// Copy variables - preserve user's order and only include user-configured variables
	if in.Variables != nil {
		systemVars := map[string]bool{
			"time":       true,
			"start_time": true,
			"end_time":   true,
		}

		apiVarsByName := make(map[string]*dashboardVariable)
		for i := range in.Variables {
			if in.Variables[i].Name != nil {
				apiVarsByName[*in.Variables[i].Name] = &in.Variables[i]
			}
		}

		var userVarNames []string
		if varConfig, ok := d.GetOk("variable"); ok {
			for _, v := range varConfig.([]interface{}) {
				vMap := v.(map[string]interface{})
				if name, ok := vMap["name"].(string); ok {
					userVarNames = append(userVarNames, name)
				}
			}
		}

		varData := make([]interface{}, 0, len(userVarNames))
		for _, name := range userVarNames {
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
