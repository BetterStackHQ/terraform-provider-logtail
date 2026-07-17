package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func validateAlert(_ context.Context, diff *schema.ResourceDiff, _ interface{}) error {
	// Only validate on create or when one of the relevant attributes changes.
	if diff.Id() != "" &&
		!diff.HasChange("alert_type") &&
		!diff.HasChange("operator") &&
		!diff.HasChange("check_period") &&
		!diff.HasChange("on_missing_data") &&
		!diff.HasChange("anomaly_training_range_days") {
		return nil
	}
	alertType := diff.Get("alert_type").(string)
	if alertType == "threshold" || alertType == "relative" {
		if diff.Get("operator").(string) == "" {
			return fmt.Errorf("operator is required for %s alerts", alertType)
		}
		if diff.Get("check_period").(int) == 0 {
			return fmt.Errorf("check_period is required for %s alerts", alertType)
		}
	}
	// Both fields are Computed, so state carries API-returned values; only what
	// is literally set in the configuration should fail the type check.
	if raw := diff.GetRawConfig(); !raw.IsNull() {
		if alertType == "anomaly_rrcf" && !raw.GetAttr("on_missing_data").IsNull() {
			return fmt.Errorf("on_missing_data is only supported for threshold and relative alerts")
		}
		if alertType != "anomaly_rrcf" && !raw.GetAttr("anomaly_training_range_days").IsNull() {
			return fmt.Errorf("anomaly_training_range_days is only supported for anomaly_rrcf alerts")
		}
	}
	return nil
}

// alertSchema contains the common alert fields shared between exploration alerts and dashboard alerts.
// Parent ID fields (exploration_id, dashboard_id, chart_id) are added by each resource separately.
var alertSchema = map[string]*schema.Schema{
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
		Description:  "The type of alert: 'threshold', 'relative', or 'anomaly_rrcf'.",
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringInSlice([]string{"threshold", "relative", "anomaly_rrcf"}, false),
	},
	"operator": {
		Description:  "The comparison operator. Required for threshold and relative alerts; not used for anomaly alerts. For threshold: 'equal', 'not_equal', 'higher_than', 'higher_than_or_equal', 'lower_than', 'lower_than_or_equal'. For relative: 'increases_by', 'decreases_by', 'changes_by'.",
		Type:         schema.TypeString,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.StringInSlice([]string{"equal", "not_equal", "higher_than", "higher_than_or_equal", "lower_than", "lower_than_or_equal", "increases_by", "decreases_by", "changes_by"}, false),
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
		Description: "The query evaluation window in seconds.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
	},
	"confirmation_period": {
		Description: "The confirmation delay in seconds before triggering.",
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
		Description: "How often to check the alert condition in seconds. Required for threshold and relative alerts; ignored for anomaly alerts, which derive their cadence from query_period.",
		Type:        schema.TypeInt,
		Optional:    true,
		Computed:    true,
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// The API does not return check_period for anomaly alerts (they use
			// query_period), so once the alert exists a configured value is not
			// real drift. This keeps imported anomaly alerts from showing a
			// perpetual diff; new alerts still send check_period on create.
			return d.Id() != "" && d.Get("alert_type").(string) == "anomaly_rrcf"
		},
	},
	"series_names": {
		Description:   "Specific series to monitor. Conflicts with series_names_except.",
		Type:          schema.TypeList,
		Optional:      true,
		Computed:      true,
		Elem:          &schema.Schema{Type: schema.TypeString},
		ConflictsWith: []string{"series_names_except"},
	},
	"series_names_except": {
		Description:   "Monitor all series except these. Conflicts with series_names.",
		Type:          schema.TypeList,
		Optional:      true,
		Computed:      true,
		Elem:          &schema.Schema{Type: schema.TypeString},
		ConflictsWith: []string{"series_names"},
	},
	"on_missing_data": {
		Description:  "What to do when the monitored query returns no data: 'treat_as_zero', 'dont_fire', 'treat_as_previous', or 'start_incident'. Only for threshold and relative alerts.",
		Type:         schema.TypeString,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.StringInSlice([]string{"treat_as_zero", "dont_fire", "treat_as_previous", "start_incident"}, false),
	},
	"source_variable": {
		Description: "Source reference (format: 'source:table_name'). If omitted, derived from the parent resource's source variable.",
		Type:        schema.TypeString,
		Optional:    true,
		Computed:    true,
	},
	"source_mode": {
		Description:  "Source selection mode: 'source_variable', 'platforms_single_source', or 'platforms_all_sources'.",
		Type:         schema.TypeString,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.StringInSlice([]string{"source_variable", "platforms_single_source", "platforms_all_sources"}, false),
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
		Description:  "Anomaly trigger mode: 'any', 'higher', or 'lower' (only for 'anomaly_rrcf' type).",
		Type:         schema.TypeString,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.StringInSlice([]string{"any", "higher", "lower"}, false),
	},
	"anomaly_training_range_days": {
		Description:  "How many days of history to train the anomaly detection on, 1-30 (only for 'anomaly_rrcf' type).",
		Type:         schema.TypeInt,
		Optional:     true,
		Computed:     true,
		ValidateFunc: validation.IntBetween(1, 30),
	},
	"paused_reason": {
		Description: "Read-only field explaining why the alert is paused (e.g., 'Manually paused', complexity issues, too many failures).",
		Type:        schema.TypeString,
		Computed:    true,
	},
	"escalation_target": {
		Description: "The escalation target for this alert. Specify either team_id/team_name OR policy_id/policy_name.",
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
				"policy_name": {
					Description: "The Better Stack escalation policy name.",
					Type:        schema.TypeString,
					Optional:    true,
				},
			},
		},
	},
	"metadata": {
		Description: "Custom metadata key-value pairs included in incident notifications. Use a plain string for a single value; for multiple values use jsonencode([...]).",
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

// alertMetadataValue represents a Better Stack alert metadata value, which may
// be either a plain string or a JSON array of strings on the wire. The API
// always returns arrays on reads but still accepts both shapes on writes.
type alertMetadataValue struct {
	isArray bool
	str     string
	arr     []string
}

func (v *alertMetadataValue) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		v.isArray = true
		v.arr = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.isArray = false
		v.str = s
		return nil
	}
	return fmt.Errorf("metadata value must be string or []string, got %s", string(data))
}

func (v alertMetadataValue) MarshalJSON() ([]byte, error) {
	if v.isArray {
		if v.arr == nil {
			return json.Marshal([]string{})
		}
		return json.Marshal(v.arr)
	}
	return json.Marshal(v.str)
}

// terraformValue normalizes a metadata value back to its Terraform schema
// representation (TypeMap of TypeString). Single-element arrays become plain
// strings; multi-element arrays are emitted as compact JSON, matching the
// shape produced by HCL's jsonencode().
func (v alertMetadataValue) terraformValue() string {
	if !v.isArray {
		return v.str
	}
	if len(v.arr) == 1 {
		return v.arr[0]
	}
	buf, err := json.Marshal(v.arr)
	if err != nil {
		return ""
	}
	return string(buf)
}

// metadataValueFromTerraform parses a Terraform metadata string into the
// wire shape. Strings that look like JSON arrays of strings are sent as
// arrays; everything else is sent as a plain string.
func metadataValueFromTerraform(s string) alertMetadataValue {
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return alertMetadataValue{isArray: true, arr: arr}
		}
	}
	return alertMetadataValue{isArray: false, str: s}
}

// alertEscalationTarget handles polymorphic response - can be null, string "current_team", or object
type alertEscalationTarget struct {
	TeamID     *int    `json:"team_id,omitempty"`
	TeamName   *string `json:"team_name,omitempty"`
	PolicyID   *int    `json:"policy_id,omitempty"`
	PolicyName *string `json:"policy_name,omitempty"`
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

type alert struct {
	Name                     *string                       `json:"name,omitempty"`
	AlertType                *string                       `json:"alert_type,omitempty"`
	Operator                 *string                       `json:"operator,omitempty"`
	Value                    *float64                      `json:"value,omitempty"`
	StringValue              *string                       `json:"string_value,omitempty"`
	QueryPeriod              *int                          `json:"query_period,omitempty"`
	ConfirmationPeriod       *int                          `json:"confirmation_period,omitempty"`
	RecoveryPeriod           *int                          `json:"recovery_period,omitempty"`
	AggregationInterval      *int                          `json:"aggregation_interval,omitempty"`
	CheckPeriod              *int                          `json:"check_period,omitempty"`
	SeriesNames              []string                      `json:"series_names,omitempty"`
	SeriesNamesExcept        []string                      `json:"series_names_except,omitempty"`
	OnMissingData            *string                       `json:"on_missing_data,omitempty"`
	SourceVariable           *string                       `json:"source_variable,omitempty"`
	SourceMode               *string                       `json:"source_mode,omitempty"`
	SourcePlatforms          []string                      `json:"source_platforms,omitempty"`
	IncidentCause            *string                       `json:"incident_cause,omitempty"`
	IncidentPerSeries        *bool                         `json:"incident_per_series,omitempty"`
	Paused                   *bool                         `json:"paused,omitempty"`
	PausedReason             *string                       `json:"paused_reason,omitempty"`
	Call                     *bool                         `json:"call,omitempty"`
	SMS                      *bool                         `json:"sms,omitempty"`
	Email                    *bool                         `json:"email,omitempty"`
	Push                     *bool                         `json:"push,omitempty"`
	CriticalAlert            *bool                         `json:"critical_alert,omitempty"`
	AnomalySensitivity       *float64                      `json:"anomaly_sensitivity,omitempty"`
	AnomalyTrigger           *string                       `json:"anomaly_trigger,omitempty"`
	AnomalyTrainingRangeDays *int                          `json:"anomaly_training_range_days,omitempty"`
	EscalationTarget         alertEscalationTargetWrapper  `json:"escalation_target,omitempty"`
	Metadata                 map[string]alertMetadataValue `json:"metadata,omitempty"`
	CreatedAt                *string                       `json:"created_at,omitempty"`
	UpdatedAt                *string                       `json:"updated_at,omitempty"`
}

type alertHTTPResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes alert  `json:"attributes"`
	} `json:"data"`
}

func loadAlert(d *schema.ResourceData) alert {
	var in alert

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
	if v, ok := d.GetOk("on_missing_data"); ok {
		s := v.(string)
		in.OnMissingData = &s
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
	in.AnomalyTrainingRangeDays = intFromResourceData(d, "anomaly_training_range_days")

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
	if v, ok := d.GetOk("series_names_except"); ok {
		list := v.([]interface{})
		names := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		in.SeriesNamesExcept = names
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
			if policyName, ok := targetMap["policy_name"].(string); ok && policyName != "" {
				target.PolicyName = &policyName
			}

			in.EscalationTarget = alertEscalationTargetWrapper{Value: target}
		}
	}

	// Load metadata
	if v, ok := d.GetOk("metadata"); ok {
		metaMap := v.(map[string]interface{})
		metadata := make(map[string]alertMetadataValue)
		for k, val := range metaMap {
			if s, ok := val.(string); ok {
				metadata[k] = metadataValueFromTerraform(s)
			}
		}
		in.Metadata = metadata
	}

	return in
}

func alertCopyAttrs(d *schema.ResourceData, in *alert) diag.Diagnostics {
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
	if in.OnMissingData != nil {
		if err := d.Set("on_missing_data", *in.OnMissingData); err != nil {
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
	if in.AnomalyTrainingRangeDays != nil {
		if err := d.Set("anomaly_training_range_days", *in.AnomalyTrainingRangeDays); err != nil {
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
	if in.SeriesNamesExcept != nil {
		if err := d.Set("series_names_except", in.SeriesNamesExcept); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}
	if in.SourcePlatforms != nil {
		if err := d.Set("source_platforms", in.SourcePlatforms); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	// Copy escalation_target. On a normal refresh we mirror back only the fields
	// already present in config/state to avoid drift (the API echoes both the id
	// and the name, but the user may have configured only one). With no prior
	// config - e.g. terraform import - we adopt the canonical identifier the API
	// returns so the target isn't silently dropped.
	if in.EscalationTarget.Value != nil {
		v := in.EscalationTarget.Value

		var hasTeamID, hasTeamName, hasPolicyID, hasPolicyName bool
		if escConfig, ok := d.GetOk("escalation_target"); ok {
			list := escConfig.([]interface{})
			if len(list) > 0 && list[0] != nil {
				targetMap := list[0].(map[string]interface{})
				if id, ok := targetMap["team_id"].(int); ok && id != 0 {
					hasTeamID = true
				}
				if name, ok := targetMap["team_name"].(string); ok && name != "" {
					hasTeamName = true
				}
				if id, ok := targetMap["policy_id"].(int); ok && id != 0 {
					hasPolicyID = true
				}
				if name, ok := targetMap["policy_name"].(string); ok && name != "" {
					hasPolicyName = true
				}
			}
		}

		targetData := make(map[string]interface{})
		if hasTeamID || hasTeamName || hasPolicyID || hasPolicyName {
			if v.TeamID != nil && hasTeamID {
				targetData["team_id"] = *v.TeamID
			}
			if v.TeamName != nil && hasTeamName {
				targetData["team_name"] = *v.TeamName
			}
			if v.PolicyID != nil && hasPolicyID {
				targetData["policy_id"] = *v.PolicyID
			}
			if v.PolicyName != nil && hasPolicyName {
				targetData["policy_name"] = *v.PolicyName
			}
		} else {
			switch {
			case v.PolicyID != nil:
				targetData["policy_id"] = *v.PolicyID
			case v.TeamID != nil:
				targetData["team_id"] = *v.TeamID
			case v.PolicyName != nil:
				targetData["policy_name"] = *v.PolicyName
			case v.TeamName != nil:
				targetData["team_name"] = *v.TeamName
			}
		}
		if len(targetData) > 0 {
			if err := d.Set("escalation_target", []interface{}{targetData}); err != nil {
				derr = append(derr, diag.FromErr(err)[0])
			}
		}
	}

	// Copy metadata - flatten polymorphic values back to strings.
	// Single-element arrays become plain strings; multi-element arrays are
	// emitted as compact JSON, matching what jsonencode([...]) produces.
	if in.Metadata != nil {
		flat := make(map[string]string, len(in.Metadata))
		for k, val := range in.Metadata {
			flat[k] = val.terraformValue()
		}
		if err := d.Set("metadata", flat); err != nil {
			derr = append(derr, diag.FromErr(err)[0])
		}
	}

	return derr
}
