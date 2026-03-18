package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var explorationAlertSchema = func() map[string]*schema.Schema {
	s := make(map[string]*schema.Schema)
	for k, v := range alertSchema {
		cp := *v
		s[k] = &cp
	}
	s["exploration_id"] = &schema.Schema{
		Description: "The ID of the exploration this alert belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	}
	return s
}()

func newExplorationAlertResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: explorationAlertCreate,
		ReadContext:   explorationAlertRead,
		UpdateContext: explorationAlertUpdate,
		DeleteContext: explorationAlertDelete,
		Importer: &schema.ResourceImporter{
			StateContext: explorationAlertImportState,
		},
		CustomizeDiff: validateAlert,
		Description:   "This resource allows you to create, modify, and delete Alerts on Explorations in Better Stack Telemetry.",
		Schema:        explorationAlertSchema,
	}
}

func explorationAlertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID := d.Get("exploration_id").(string)
	in := loadAlert(d)

	var out alertHTTPResponse
	if err := resourceCreate(ctx, meta, fmt.Sprintf("/api/v2/explorations/%s/alerts", url.PathEscape(explorationID)), &in, &out); err != nil {
		return err
	}

	// Set composite ID: exploration_id/alert_id
	d.SetId(fmt.Sprintf("%s/%s", explorationID, out.Data.ID))
	return alertCopyAttrs(d, &out.Data.Attributes)
}

func explorationAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var out alertHTTPResponse
	if diags, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(),
		fmt.Sprintf("/api/v2/explorations/%s/alerts/%s", url.PathEscape(explorationID), url.PathEscape(alertID)), &out); diags != nil {
		return diags
	} else if !ok {
		d.SetId("") // Force "create" on 404.
		return nil
	}

	// Set exploration_id for state
	if err := d.Set("exploration_id", explorationID); err != nil {
		return diag.FromErr(err)
	}

	return alertCopyAttrs(d, &out.Data.Attributes)
}

func explorationAlertUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	in := loadAlert(d)

	if diags := resourceUpdate(ctx, meta,
		fmt.Sprintf("/api/v2/explorations/%s/alerts/%s", url.PathEscape(explorationID), url.PathEscape(alertID)), &in); diags != nil {
		return diags
	}
	// Read back the resource to get computed values
	return explorationAlertRead(ctx, d, meta)
}

func explorationAlertDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceDelete(ctx, meta,
		fmt.Sprintf("/api/v2/explorations/%s/alerts/%s", url.PathEscape(explorationID), url.PathEscape(alertID)))
}

func explorationAlertImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// Expected format: exploration_id/alert_id
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return nil, err
	}

	if err := d.Set("exploration_id", explorationID); err != nil {
		return nil, err
	}

	// Re-set the ID to ensure consistent format
	d.SetId(fmt.Sprintf("%s/%s", explorationID, alertID))

	return []*schema.ResourceData{d}, nil
}

func parseExplorationAlertID(id string) (explorationID, alertID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid alert ID format %q, expected 'exploration_id/alert_id'", id)
	}
	return parts[0], parts[1], nil
}
