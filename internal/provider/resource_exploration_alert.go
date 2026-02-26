package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var explorationAlertSchema = map[string]*schema.Schema{
	"exploration_id": {
		Description: "The ID of the exploration this alert belongs to.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"id": {
		Description: "The ID of this alert.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"name": {
		Description: "The name of this alert.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"alert_type": {
		Description: "The type of alert: 'threshold', 'relative', or 'anomaly_rrcf'.",
		Type:        schema.TypeString,
		Required:    true,
	},
	"operator": {
		Description: "The comparison operator. For threshold: 'equal', 'not_equal', 'higher_than', 'higher_than_or_equal', 'lower_than', 'lower_than_or_equal'. For relative: 'increases_by', 'decreases_by', 'changes_by'. Not required for anomaly alerts.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"value": {
		Description: "The numeric threshold value. Required for threshold and relative alerts.",
		Type:        schema.TypeFloat,
		Optional:    true,
		Computed:    true,
	},
	"string_value": {
		Description: "The string threshold value (only for threshold alerts with 'equal' or 'not_equal' operators).",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"query_period": {
		Description: "The query evaluation window in seconds (default: 60).",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"confirmation_period": {
		Description: "The confirmation delay in seconds before triggering (required, >= 0).",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"recovery_period": {
		Description: "The recovery delay in seconds.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"aggregation_interval": {
		Description: "The data aggregation interval in seconds.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"check_period": {
		Description: "How often to check the alert condition in seconds.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"series_names": {
		Description: "Specific series to monitor.",
		Type:        schema.TypeList,
		Optional:    true,
		Computed:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
	},
	"source_variable": {
		Description: "Source reference (format: 'source:table_name'). If omitted, derived from exploration's source variable.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"source_mode": {
		Description: "Source selection mode: 'source_variable', 'platforms_single_source', or 'platforms_all_sources'.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"source_platforms": {
		Description: "Platform filters (used when source_mode is 'platforms_*').",
		Type:        schema.TypeList,
		Optional:    true,
		Computed:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
	},
	"incident_cause": {
		Description: "Incident description template (supports {{variable}} interpolation).",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"incident_per_series": {
		Description: "Create separate incidents per series.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"paused": {
		Description: "Whether the alert is paused.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"call": {
		Description: "Enable phone call notifications.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"sms": {
		Description: "Enable SMS notifications.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"email": {
		Description: "Enable email notifications.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"push": {
		Description: "Enable push notifications.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"critical_alert": {
		Description: "Mark as critical alert (bypasses quiet hours).",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"anomaly_sensitivity": {
		Description: "Anomaly detection sensitivity 0-100 (only for 'anomaly_rrcf' type, lower = more sensitive).",
		Type:        schema.TypeFloat,
		Optional:    true,
		Computed:    true,
	},
	"anomaly_trigger": {
		Description: "Anomaly trigger mode: 'any', 'higher', or 'lower' (only for 'anomaly_rrcf' type).",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"paused_reason": {
		Description: "Read-only field explaining why the alert is paused (e.g., 'Manually paused', complexity issues, too many failures).",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"escalation_target": {
		Description: "The escalation target for this alert. Specify either team_id/team_name OR policy_id.",
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"team_id": {
					Description: "The Better Stack team ID to escalate to.",
					Type:        schema.TypeInt,
					Optional:    true,
				},
				"team_name": {
					Description: "The Better Stack team name to escalate to.",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"policy_id": {
					Description: "The Better Stack escalation policy ID.",
					Type:        schema.TypeInt,
					Optional:    true,
				},
			},
		},
	},
	"metadata": {
		Description: "Custom metadata key-value pairs included in incident notifications.",
		Type:        schema.TypeMap,
		Optional:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
	},
	"created_at": {
		Description: "The time when this alert was created.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
	"updated_at": {
		Description: "The time when this alert was updated.",
		Type:        schema.TypeString,
		Optional:    false,
		Computed:    true,
	},
}

func newExplorationAlertResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: explorationAlertCreate,
		ReadContext:   explorationAlertRead,
		UpdateContext: explorationAlertUpdate,
		DeleteContext: explorationAlertDelete,
		Importer: &schema.ResourceImporter{
			StateContext: explorationAlertImportState,
		},
		Description: "This resource allows you to create, modify, and delete Alerts on Explorations in Better Stack Telemetry.",
		Schema:      explorationAlertSchema,
	}
}

// API structs

// alertEscalationTarget handles polymorphic response - can be null, string "current_team", or object
type alertEscalationTarget struct {
	TeamID   *int    `json:"team_id,omitempty"`
	TeamName *string `json:"team_name,omitempty"`
	PolicyID *int    `json:"policy_id,omitempty"`
}

// alertEscalationTargetWrapper handles the polymorphic escalation_target field
type alertEscalationTargetWrapper struct {
	Value *alertEscalationTarget
}

func (w *alertEscalationTargetWrapper) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first (e.g., "current_team")
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// String value like "current_team" means no explicit target set
		w.Value = nil
		return nil
	}

	// Try to unmarshal as object
	var target alertEscalationTarget
	if err := json.Unmarshal(data, &target); err != nil {
		return err
	}
	w.Value = &target
	return nil
}

func (w alertEscalationTargetWrapper) MarshalJSON() ([]byte, error) {
	if w.Value == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(w.Value)
}

type explorationAlert struct {
	Name                *string                      `json:"name,omitempty"`
	AlertType           *string                      `json:"alert_type,omitempty"`
	Operator            *string                      `json:"operator,omitempty"`
	Value               *float64                     `json:"value,omitempty"`
	StringValue         *string                      `json:"string_value,omitempty"`
	QueryPeriod         *int                         `json:"query_period,omitempty"`
	ConfirmationPeriod  *int                         `json:"confirmation_period,omitempty"`
	RecoveryPeriod      *int                         `json:"recovery_period,omitempty"`
	AggregationInterval *int                         `json:"aggregation_interval,omitempty"`
	CheckPeriod         *int                         `json:"check_period,omitempty"`
	SeriesNames         []string                     `json:"series_names,omitempty"`
	SourceVariable      *string                      `json:"source_variable,omitempty"`
	SourceMode          *string                      `json:"source_mode,omitempty"`
	SourcePlatforms     []string                     `json:"source_platforms,omitempty"`
	IncidentCause       *string                      `json:"incident_cause,omitempty"`
	IncidentPerSeries   *bool                        `json:"incident_per_series,omitempty"`
	Paused              *bool                        `json:"paused,omitempty"`
	PausedReason        *string                      `json:"paused_reason,omitempty"`
	Call                *bool                        `json:"call,omitempty"`
	SMS                 *bool                        `json:"sms,omitempty"`
	Email               *bool                        `json:"email,omitempty"`
	Push                *bool                        `json:"push,omitempty"`
	CriticalAlert       *bool                        `json:"critical_alert,omitempty"`
	AnomalySensitivity  *float64                     `json:"anomaly_sensitivity,omitempty"`
	AnomalyTrigger      *string                      `json:"anomaly_trigger,omitempty"`
	EscalationTarget    alertEscalationTargetWrapper `json:"escalation_target,omitempty"`
	Metadata            map[string]string            `json:"metadata,omitempty"`
	CreatedAt           *string                      `json:"created_at,omitempty"`
	UpdatedAt           *string                      `json:"updated_at,omitempty"`
}

type explorationAlertHTTPResponse struct {
	Data struct {
		ID         string           `json:"id"`
		Attributes explorationAlert `json:"attributes"`
	} `json:"data"`
}

func explorationAlertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID := d.Get("exploration_id").(string)
	in := loadExplorationAlert(d)

	var out explorationAlertHTTPResponse
	if err := resourceCreate(ctx, meta, fmt.Sprintf("/api/v2/explorations/%s/alerts", url.PathEscape(explorationID)), &in, &out); err != nil {
		return err
	}

	// Set composite ID: exploration_id/alert_id
	d.SetId(fmt.Sprintf("%s/%s", explorationID, out.Data.ID))
	return explorationAlertCopyAttrs(d, &out.Data.Attributes)
}

func explorationAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var out explorationAlertHTTPResponse
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

	return explorationAlertCopyAttrs(d, &out.Data.Attributes)
}

func explorationAlertUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	explorationID, alertID, err := parseExplorationAlertID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	in := loadExplorationAlert(d)

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

func loadExplorationAlert(d *schema.ResourceData) explorationAlert {
	var in explorationAlert

	// Load string fields
	if v, ok := d.GetOk("name"); ok {
		s := v.(string)
		in.Name = &s
	}
	if v, ok := d.GetOk("alert_type"); ok {
		s := v.(string)
		in.AlertType = &s
	}
	if v, ok := d.GetOk("operator"); ok {
		s := v.(string)
		in.Operator = &s
	}
	if v, ok := d.GetOk("string_value"); ok {
		s := v.(string)
		in.StringValue = &s
	}
	if v, ok := d.GetOk("source_variable"); ok {
		s := v.(string)
		in.SourceVariable = &s
	}
	if v, ok := d.GetOk("source_mode"); ok {
		s := v.(string)
		in.SourceMode = &s
	}
	if v, ok := d.GetOk("incident_cause"); ok {
		s := v.(string)
		in.IncidentCause = &s
	}
	if v, ok := d.GetOk("anomaly_trigger"); ok {
		s := v.(string)
		in.AnomalyTrigger = &s
	}

	// Load float fields - use helper to allow 0 values
	in.Value = floatFromResourceData(d, "value")
	in.AnomalySensitivity = floatFromResourceData(d, "anomaly_sensitivity")

	// Load int fields - use helper to allow 0 values
	in.QueryPeriod = intFromResourceData(d, "query_period")
	in.ConfirmationPeriod = intFromResourceData(d, "confirmation_period")
	in.RecoveryPeriod = intFromResourceData(d, "recovery_period")
	in.AggregationInterval = intFromResourceData(d, "aggregation_interval")
	in.CheckPeriod = intFromResourceData(d, "check_period")

	// Load bool fields - use helper to allow false values
	in.Paused = boolFromResourceData(d, "paused")
	in.IncidentPerSeries = boolFromResourceData(d, "incident_per_series")
	in.Call = boolFromResourceData(d, "call")
	in.SMS = boolFromResourceData(d, "sms")
	in.Email = boolFromResourceData(d, "email")
	in.Push = boolFromResourceData(d, "push")
	in.CriticalAlert = boolFromResourceData(d, "critical_alert")

	// Load string arrays
	if v, ok := d.GetOk("series_names"); ok {
		list := v.([]interface{})
		names := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		in.SeriesNames = names
	}
	if v, ok := d.GetOk("source_platforms"); ok {
		list := v.([]interface{})
		platforms := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				platforms = append(platforms, s)
			}
		}
		in.SourcePlatforms = platforms
	}

	// Load escalation_target
	if v, ok := d.GetOk("escalation_target"); ok {
		list := v.([]interface{})
		if len(list) > 0 {
			targetMap := list[0].(map[string]interface{})
			target := &alertEscalationTarget{}

			if teamID, ok := targetMap["team_id"].(int); ok && teamID != 0 {
				target.TeamID = &teamID
			}
			if teamName, ok := targetMap["team_name"].(string); ok && teamName != "" {
				target.TeamName = &teamName
			}
			if policyID, ok := targetMap["policy_id"].(int); ok && policyID != 0 {
				target.PolicyID = &policyID
			}

			in.EscalationTarget = alertEscalationTargetWrapper{Value: target}
		}
	}

	// Load metadata
	if v, ok := d.GetOk("metadata"); ok {
		metaMap := v.(map[string]interface{})
		metadata := make(map[string]string)
		for k, val := range metaMap {
			if s, ok := val.(string); ok {
				metadata[k] = s
			}
		}
		in.Metadata = metadata
	}

	return in
}

func explorationAlertCopyAttrs(d *schema.ResourceData, in *explorationAlert) diag.Diagnostics {
	var derr diag.Diagnostics

	// Copy string fields
	if in.Name != nil {
		if err := d.Set("name", *in.Name); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.AlertType != nil {
		if err := d.Set("alert_type", *in.AlertType); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Operator != nil {
		if err := d.Set("operator", *in.Operator); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.StringValue != nil {
		if err := d.Set("string_value", *in.StringValue); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.SourceVariable != nil {
		if err := d.Set("source_variable", *in.SourceVariable); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.SourceMode != nil {
		if err := d.Set("source_mode", *in.SourceMode); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.IncidentCause != nil {
		if err := d.Set("incident_cause", *in.IncidentCause); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.AnomalyTrigger != nil {
		if err := d.Set("anomaly_trigger", *in.AnomalyTrigger); err != nil {
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

	// Copy float fields
	if in.Value != nil {
		if err := d.Set("value", *in.Value); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.AnomalySensitivity != nil {
		if err := d.Set("anomaly_sensitivity", *in.AnomalySensitivity); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy int fields
	if in.QueryPeriod != nil {
		if err := d.Set("query_period", *in.QueryPeriod); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.ConfirmationPeriod != nil {
		if err := d.Set("confirmation_period", *in.ConfirmationPeriod); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.RecoveryPeriod != nil {
		if err := d.Set("recovery_period", *in.RecoveryPeriod); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.AggregationInterval != nil {
		if err := d.Set("aggregation_interval", *in.AggregationInterval); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.CheckPeriod != nil {
		if err := d.Set("check_period", *in.CheckPeriod); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy bool fields
	if in.Paused != nil {
		if err := d.Set("paused", *in.Paused); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.PausedReason != nil {
		if err := d.Set("paused_reason", *in.PausedReason); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.IncidentPerSeries != nil {
		if err := d.Set("incident_per_series", *in.IncidentPerSeries); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Call != nil {
		if err := d.Set("call", *in.Call); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.SMS != nil {
		if err := d.Set("sms", *in.SMS); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Email != nil {
		if err := d.Set("email", *in.Email); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.Push != nil {
		if err := d.Set("push", *in.Push); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.CriticalAlert != nil {
		if err := d.Set("critical_alert", *in.CriticalAlert); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy arrays
	if in.SeriesNames != nil {
		if err := d.Set("series_names", in.SeriesNames); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.SourcePlatforms != nil {
		if err := d.Set("source_platforms", in.SourcePlatforms); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy escalation_target
	if in.EscalationTarget.Value != nil {
		targetData := make(map[string]interface{})
		if in.EscalationTarget.Value.TeamID != nil {
			targetData["team_id"] = *in.EscalationTarget.Value.TeamID
		}
		if in.EscalationTarget.Value.TeamName != nil {
			targetData["team_name"] = *in.EscalationTarget.Value.TeamName
		}
		if in.EscalationTarget.Value.PolicyID != nil {
			targetData["policy_id"] = *in.EscalationTarget.Value.PolicyID
		}
		if err := d.Set("escalation_target", []interface{}{targetData}); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy metadata
	if in.Metadata != nil {
		if err := d.Set("metadata", in.Metadata); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}
