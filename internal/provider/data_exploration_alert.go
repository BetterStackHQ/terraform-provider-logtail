package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newExplorationAlertDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)

	for k, v := range explorationAlertSchema {
		cp := *v
		switch k {
		case "exploration_id":
			// exploration_id is required for lookup
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
			cp.ForceNew = false
		case "name":
			// Name is used for lookup - make it required
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		default:
			// All other fields become computed (read-only)
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
			cp.ForceNew = false
			cp.MaxItems = 0
		}
		s[k] = &cp
	}

	return &schema.Resource{
		ReadContext: explorationAlertLookup,
		Description: "This data source allows you to get information about an Alert on an Exploration in Better Stack Telemetry.",
		Schema:      s,
	}
}

type explorationAlertsHTTPResponse struct {
	Data []struct {
		ID         string           `json:"id"`
		Attributes explorationAlert `json:"attributes"`
	} `json:"data"`
}

func explorationAlertLookup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID := d.Get("exploration_id").(string)
	name := d.Get("name").(string)
	c := meta.(*client)

	// Fetch all alerts for the exploration (no pagination for alerts)
	res, err := c.Get(ctx, fmt.Sprintf("/api/v2/explorations/%s/alerts", url.PathEscape(explorationID)))
	if err != nil {
		return diag.FromErr(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	body, err := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return diag.Errorf("GET %s returned %d: %s", res.Request.URL.String(), res.StatusCode, string(body))
	}
	if err != nil {
		return diag.FromErr(err)
	}

	var out explorationAlertsHTTPResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return diag.FromErr(err)
	}

	// Find the alert by name
	for _, item := range out.Data {
		if item.Attributes.Name != nil && *item.Attributes.Name == name {
			// Set composite ID: exploration_id/alert_id
			d.SetId(fmt.Sprintf("%s/%s", explorationID, item.ID))

			// Set exploration_id explicitly
			if err := d.Set("exploration_id", explorationID); err != nil {
				return diag.FromErr(err)
			}

			return explorationAlertCopyAttrs(d, &item.Attributes)
		}
	}

	return diag.Errorf("Alert with name %q not found in exploration %s", name, explorationID)
}
