package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type dashboardListItem struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes dashboardListAttrs `json:"attributes"`
}

type dashboardListAttrs struct {
	TeamID    *int    `json:"team_id,omitempty"`
	TeamName  *string `json:"team_name,omitempty"`
	Name      *string `json:"name,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

type templateListItem struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes templateListAttrs `json:"attributes"`
}

type templateListAttrs struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

type dashboardPageHTTPResponse struct {
	Data       []dashboardListItem `json:"data"`
	Pagination struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Prev  string `json:"prev"`
		Next  string `json:"next"`
	} `json:"pagination"`
}

type templatePageHTTPResponse struct {
	Data []templateListItem `json:"data"`
}

// formatAvailableNames creates a sorted, deduplicated, quoted list of names for error messages
func formatAvailableNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	var uniqueNames []string
	for name := range nameSet {
		uniqueNames = append(uniqueNames, name)
	}
	sort.Strings(uniqueNames)
	for i, name := range uniqueNames {
		uniqueNames[i] = fmt.Sprintf("%q", name)
	}
	return strings.Join(uniqueNames, ", ")
}

func newDashboardTemplateDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDashboardTemplateRead,
		Description: "This data source allows you to look up dashboard templates by ID or name. For more information about dashboard templates check https://betterstack.com/dashboards/",
		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID of the dashboard template to retrieve.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"name": {
				Description: "The name of the dashboard template.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"data": {
				Description: "The dashboard template configuration data in JSON format.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func newDashboardDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDashboardRead,
		Description: "This data source allows you to look up existing dashboards using their ID or name. For more information about the Dashboard API check https://betterstack.com/docs/logs/api/dashboards/",
		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID of the dashboard to retrieve.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"name": {
				Description: "The name of the dashboard.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"data": {
				Description: "The dashboard configuration data in JSON format.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"team_name": {
				Description: "The team name associated with the dashboard.",
				Type:        schema.TypeString,
				Optional:    true,
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
		},
	}
}

func dataSourceDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	name := d.Get("name").(string)
	teamName := d.Get("team_name").(string)

	// Validate input: either id or name must be provided
	if id == "" && name == "" {
		return diag.Errorf("either id or name must be specified")
	}

	// Fetch function for getting dashboards list
	fetch := func(page int) (*dashboardPageHTTPResponse, error) {
		path := fmt.Sprintf("/api/v2/dashboards?page=%d", page)
		res, err := meta.(*client).Get(ctx, path)
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
		var tr dashboardPageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}

	// If ID is provided, look up directly by ID
	if id != "" {
		// First, verify the dashboard exists in the list and get its details
		// We need to search through all pages to find the dashboard
		var dashboardExists bool
		var dashboardName string
		var dashboardTeamName *string

		page := 1
		for {
			res, err := fetch(page)
			if err != nil {
				return diag.FromErr(err)
			}

			for _, e := range res.Data {
				if e.ID == id {
					dashboardExists = true
					if e.Attributes.Name != nil {
						dashboardName = *e.Attributes.Name
					}
					dashboardTeamName = e.Attributes.TeamName
					break
				}
			}

			if dashboardExists || res.Pagination.Next == "" {
				break
			}
			page++
		}

		if !dashboardExists {
			return diag.Errorf("dashboard with ID %s not found in dashboards list", id)
		}

		// If both ID and name are provided, validate they match
		if name != "" && !strings.EqualFold(dashboardName, name) {
			return diag.Errorf("dashboard with ID %s has name %q, but requested name is %q", id, dashboardName, name)
		}

		// If team_name is provided, validate it matches
		if teamName != "" && (dashboardTeamName == nil || !strings.EqualFold(*dashboardTeamName, teamName)) {
			actualTeamName := ""
			if dashboardTeamName != nil {
				actualTeamName = *dashboardTeamName
			}
			return diag.Errorf("dashboard with ID %s has team_name %q, but requested team_name is %q", id, actualTeamName, teamName)
		}

		var out dashboardHTTPResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(id)), &out); !ok {
			return diag.Errorf("dashboard %q was found but couldn't be exported: %v", dashboardName, err)
		}

		d.SetId(id)

		// Set the attributes from the response
		if out.Data.Attributes.Name != nil {
			if err := d.Set("name", *out.Data.Attributes.Name); err != nil {
				return diag.FromErr(err)
			}
		}

		// Get the full dashboard data using the export API
		var exportOut dashboardExportResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s/export", url.PathEscape(id)), &exportOut); !ok {
			return diag.Errorf("dashboard %q was found but couldn't be exported: %v", dashboardName, err)
		}

		// Convert the data map back to JSON string
		dataBytes, err := json.Marshal(exportOut.Data)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("data", string(dataBytes)); err != nil {
			return diag.FromErr(err)
		}

		// Set additional attributes from basic response
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

		return nil
	}

	// If name is provided, look up by name

	// Collect all matching dashboards across all pages
	var matchingDashboards []dashboardListItem
	page := 1
	for {
		res, err := fetch(page)
		if err != nil {
			return diag.FromErr(err)
		}
		for _, e := range res.Data {
			// Check if name matches
			if e.Attributes.Name != nil && *e.Attributes.Name == name {
				// Check if team_name matches (if specified)
				if teamName != "" && (e.Attributes.TeamName == nil || *e.Attributes.TeamName != teamName) {
					continue
				}
				matchingDashboards = append(matchingDashboards, e)
			}
		}
		page++
		if res.Pagination.Next == "" {
			break
		}
	}

	// Handle multiple matches
	if len(matchingDashboards) > 1 {
		var ids []string
		for _, dashboard := range matchingDashboards {
			ids = append(ids, dashboard.ID)
		}
		return diag.Errorf("multiple dashboards found with the name %q - use ID lookup instead, available dashboard IDs: %s", name, strings.Join(ids, ", "))
	}

	// Handle single match
	if len(matchingDashboards) == 1 {
		e := matchingDashboards[0]

		// Get the full dashboard details using basic API
		var out dashboardHTTPResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s", url.PathEscape(e.ID)), &out); err != nil {
			if !ok {
				// Dashboard not accessible
				return diag.Errorf("dashboard with ID %s not accessible", e.ID)
			}
			return err
		}

		d.SetId(e.ID)

		// Set the attributes from the response
		if out.Data.Attributes.Name != nil {
			if err := d.Set("name", *out.Data.Attributes.Name); err != nil {
				return diag.FromErr(err)
			}
		}

		// Get the full dashboard data using the export API
		var exportOut dashboardExportResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s/export", url.PathEscape(e.ID)), &exportOut); !ok {
			return diag.Errorf("dashboard %q was found but couldn't be exported: %v", *e.Attributes.Name, err)
		} else {
			// Convert the data map back to JSON string
			dataBytes, err := json.Marshal(exportOut.Data)
			if err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("data", string(dataBytes)); err != nil {
				return diag.FromErr(err)
			}
		}

		// Set additional attributes from basic response
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
		return nil
	}

	if d.Id() == "" {
		return diag.Errorf("no dashboard found with name %q", name)
	}

	return nil
}

func dataSourceDashboardTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	name := d.Get("name").(string)

	// Validate input: either id or name must be provided
	if id == "" && name == "" {
		return diag.Errorf("either id or name must be specified")
	}

	// Fetch function for getting templates list
	fetch := func() (*templatePageHTTPResponse, error) {
		path := "/api/v2/dashboards/templates"
		res, err := meta.(*client).Get(ctx, path)
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
		var tr templatePageHTTPResponse
		return &tr, json.Unmarshal(body, &tr)
	}

	// If ID is provided, look up directly by ID
	if id != "" {
		// First, verify the template exists in the list
		res, err := fetch()
		if err != nil {
			return diag.FromErr(err)
		}

		// Check if the ID exists in the templates list
		var templateExists bool
		var templateName string
		for _, e := range res.Data {
			if e.ID == id {
				templateExists = true
				if e.Attributes.Name != nil {
					templateName = *e.Attributes.Name
				}
				break
			}
		}

		if !templateExists {
			return diag.Errorf("dashboard template with ID %s not found in templates list", id)
		}

		// If both ID and name are provided, validate they match
		if name != "" && !strings.EqualFold(templateName, name) {
			return diag.Errorf("dashboard template with ID %s has name %q, but requested name is %q", id, templateName, name)
		}

		// Get the full template details using export API
		// Try telemetry base URL first, then warehouse base URL for templates
		var out dashboardExportResponse
		if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s/export", url.PathEscape(id)), &out); !ok {
			return diag.Errorf("dashboard template %q was found but couldn't be exported: %v", templateName, err)
		}

		d.SetId(id)

		// Set the attributes from the export response
		if err := d.Set("name", out.Name); err != nil {
			return diag.FromErr(err)
		}

		// Convert the data map back to JSON string
		dataBytes, err := json.Marshal(out.Data)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("data", string(dataBytes)); err != nil {
			return diag.FromErr(err)
		}

		return nil
	}

	// If name is provided, look up by name
	res, err := fetch()
	if err != nil {
		return diag.FromErr(err)
	}

	// Collect available template names for error messages
	var availableNames []string
	for _, e := range res.Data {
		if e.Attributes.Name != nil {
			availableNames = append(availableNames, *e.Attributes.Name)
		}
	}

	// Collect all matching templates
	var matchingTemplates []templateListItem
	for _, e := range res.Data {
		// Check if name matches (case-insensitive)
		if e.Attributes.Name != nil && strings.EqualFold(*e.Attributes.Name, name) {
			matchingTemplates = append(matchingTemplates, e)
		}
	}

	// Handle multiple matches
	if len(matchingTemplates) > 1 {
		var ids []string
		for _, tmpl := range matchingTemplates {
			ids = append(ids, tmpl.ID)
		}
		return diag.Errorf("multiple dashboard templates found with the name %q - use ID lookup instead, available template IDs: %s", name, strings.Join(ids, ", "))
	}

	// Handle single match
	if len(matchingTemplates) == 1 {
		// Found exactly one template with this name - proceed
		e := matchingTemplates[0]
		d.SetId(e.ID)

		// Get the full template details using export API
		var out dashboardExportResponse
		if diagErr, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/dashboards/%s/export", url.PathEscape(e.ID)), &out); !ok {
			return diag.Errorf("dashboard template %q was found but couldn't be exported: %v", *e.Attributes.Name, diagErr)
		}

		// Set the attributes from the export response
		if err := d.Set("name", out.Name); err != nil {
			return diag.FromErr(err)
		}

		// Convert the data map back to JSON string
		dataBytes, err := json.Marshal(out.Data)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("data", string(dataBytes)); err != nil {
			return diag.FromErr(err)
		}

		// Found the template, no need to continue searching
		return nil
	}

	availableStr := formatAvailableNames(availableNames)
	return diag.Errorf("no dashboard template found with name %q - available templates: %s", name, availableStr)
}
