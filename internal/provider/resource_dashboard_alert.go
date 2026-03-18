package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var dashboardAlertSchema = func() map[string]*schema.Schema {
	s := make(map[string]*schema.Schema)
	for k, v := range alertSchema {
		cp := *v
		s[k] = &cp
	}
	s["dashboard_id"] = &schema.Schema{
		Description: "The ID of the dashboard this alert belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	}
	s["chart_id"] = &schema.Schema{
		Description: "The ID of the chart this alert belongs to. Accepts either a bare chart ID or a composite dashboard_id/chart_id from logtail_dashboard_chart resources.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// Suppress diff when composite ID (e.g. "1/10") is compared to bare ID (e.g. "10")
			return extractBareID(old) == extractBareID(new)
		},
	}
	return s
}()

func newDashboardAlertResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: dashboardAlertCreate,
		ReadContext:   dashboardAlertRead,
		UpdateContext: dashboardAlertUpdate,
		DeleteContext: dashboardAlertDelete,
		Importer: &schema.ResourceImporter{
			StateContext: dashboardAlertImportState,
		},
		CustomizeDiff: validateAlert,
		Description:   "This resource allows you to create, modify, and delete Alerts on Dashboard Charts in Better Stack Telemetry.",
		Schema:        dashboardAlertSchema,
	}
}

// extractBareID extracts the bare resource ID from a potentially composite ID.
// For example, "1/10" returns "10", while "10" returns "10".
// This is needed because logtail_dashboard_chart.id returns "dashboard_id/chart_id".
func extractBareID(id string) string {
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		return id[idx+1:]
	}
	return id
}

func dashboardAlertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID := d.Get("dashboard_id").(string)
	chartID := extractBareID(d.Get("chart_id").(string))
	in := loadAlert(d)

	var out alertHTTPResponse
	if err := resourceCreate(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s/alerts", url.PathEscape(dashboardID), url.PathEscape(chartID)), &in, &out); err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%s/%s/%s", dashboardID, chartID, out.Data.ID))
	return alertCopyAttrs(d, &out.Data.Attributes)
}

func dashboardAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, alertID, err := parseDashboardAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var out alertHTTPResponse
	if diags, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(),
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s/alerts/%s",
			url.PathEscape(dashboardID), url.PathEscape(chartID), url.PathEscape(alertID)), &out); diags != nil {
		return diags
	} else if !ok {
		d.SetId("")
		return nil
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("chart_id", chartID); err != nil {
		return diag.FromErr(err)
	}

	return alertCopyAttrs(d, &out.Data.Attributes)
}

func dashboardAlertUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, alertID, err := parseDashboardAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	in := loadAlert(d)

	if diags := resourceUpdate(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s/alerts/%s",
			url.PathEscape(dashboardID), url.PathEscape(chartID), url.PathEscape(alertID)), &in); diags != nil {
		return diags
	}
	// Normalize chart_id in state to bare ID
	if err := d.Set("chart_id", chartID); err != nil {
		return diag.FromErr(err)
	}
	return dashboardAlertRead(ctx, d, meta)
}

func dashboardAlertDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboardID, chartID, alertID, err := parseDashboardAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceDelete(ctx, meta,
		fmt.Sprintf("/api/v2/dashboards/%s/charts/%s/alerts/%s",
			url.PathEscape(dashboardID), url.PathEscape(chartID), url.PathEscape(alertID)))
}

func dashboardAlertImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	dashboardID, chartID, alertID, err := parseDashboardAlertID(d.Id())
	if err != nil {
		return nil, err
	}

	if err := d.Set("dashboard_id", dashboardID); err != nil {
		return nil, err
	}
	if err := d.Set("chart_id", chartID); err != nil {
		return nil, err
	}

	d.SetId(fmt.Sprintf("%s/%s/%s", dashboardID, chartID, alertID))
	return []*schema.ResourceData{d}, nil
}

func parseDashboardAlertID(id string) (dashboardID, chartID, alertID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid dashboard alert ID format %q, expected 'dashboard_id/chart_id/alert_id'", id)
	}
	return parts[0], parts[1], parts[2], nil
}
