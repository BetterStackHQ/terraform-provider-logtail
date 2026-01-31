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
		Description: "The dashboard configuration data as a JSON string. Any change will re-create the dashboard. See [Dashboard Import API docs](https://betterstack.com/docs/logs/api/dashboards/import/) for details.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"team_name": {
		Description: "The team name to associate with the dashboard when using a global API token.",
		Type:        schema.TypeString,
		Optional:    true,
		ForceNew:    true,
	},
	"team_id": {
		Description: "The team ID of the dashboard.",
		Type:        schema.TypeInt,
		Computed:    true,
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

type dashboard struct {
	Name      *string     `json:"name,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	TeamName  *string     `json:"team_name,omitempty"`
	TeamId    *int        `json:"team_id,omitempty"`
	CreatedAt *string     `json:"created_at,omitempty"`
	UpdatedAt *string     `json:"updated_at,omitempty"`
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
		CustomizeDiff: validateDashboard,
		Description:   "This resource allows you to create and manage dashboards using the import/export API. For more information about the Dashboard API check https://betterstack.com/docs/logs/api/dashboards/",
		Schema:        dashboardSchema,
	}
}

func dashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Since data is ForceNew and other fields are read-only, this function should be called only when dashboard is renamed
	// It might be unintuitive to ForceNew because of a name change, rather navigate people to rename it in Better Stack
	return diag.Errorf("dashboard updates are not supported - please rename the dashboard in Better Stack")
}

func dashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// First, get the basic dashboard info to populate readonly fields
	var basicOut dashboardHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(d.Id())), &basicOut); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}

	// Set basic readonly attributes
	if basicOut.Data.Attributes.Name != nil {
		if err := d.Set("name", *basicOut.Data.Attributes.Name); err != nil {
			return diag.FromErr(err)
		}
	}
	if basicOut.Data.Attributes.TeamId != nil {
		if err := d.Set("team_id", *basicOut.Data.Attributes.TeamId); err != nil {
			return diag.FromErr(err)
		}
	}
	if basicOut.Data.Attributes.CreatedAt != nil {
		if err := d.Set("created_at", basicOut.Data.Attributes.CreatedAt); err != nil {
			return diag.FromErr(err)
		}
	}
	if basicOut.Data.Attributes.UpdatedAt != nil {
		if err := d.Set("updated_at", basicOut.Data.Attributes.UpdatedAt); err != nil {
			return diag.FromErr(err)
		}
	}

	// Note: data field is write-only and never updated from the API

	return nil
}

func dashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceDelete(ctx, meta, fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(d.Id())))
}

func dashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var in dashboard

	// Handle name and team_name normally
	load(d, "name", &in.Name)
	load(d, "team_name", &in.TeamName)

	userData := d.Get("data").(string)

	// Parse the user-provided data JSON string
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

	// Set computed attributes from the response
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

	// Set the data for the resource
	if err := d.Set("data", userData); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func validateDashboard(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
	data := diff.Get("data").(string)

	// Validate that data is provided
	if data == "" {
		return fmt.Errorf("The 'data' field is required and must contain the dashboard configuration as a JSON string.")
	}

	// Validate JSON format for data field
	var jsonData interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		return fmt.Errorf("The 'data' field must contain valid JSON. Please ensure the dashboard configuration is properly formatted JSON. Error: %v", err)
	}

	return nil
}
