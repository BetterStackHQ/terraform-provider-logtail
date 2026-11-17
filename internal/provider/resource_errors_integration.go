package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Error tracking integrations (Slack, Microsoft Teams, Linear, Jira) are connected through OAuth in the Better Stack UI,
// so the API lists, shows and updates them but can't create or delete one. Each resource adopts an existing integration
// by its ID: "create" applies the configured settings to it, "delete" only drops it from state and leaves it connected.
//
// Every setting is Optional+Computed: one left out of the configuration stays as it is in Better Stack and is read back,
// so adopting an integration changes only what the configuration names.

type errorsIntegrationKind struct {
	// e.g. "logtail_errors_slack_integration"
	resourceName string
	// e.g. "/api/v1/slack-integrations"
	path string
	// e.g. "Slack", as the Better Stack UI names the integration
	product string
	// The "Notify me when" toggles this kind offers.
	notifications []string
	// Whether the integration filters by application at the top level (Microsoft Teams routes per application instead).
	applications bool
	// What the integration does, for the resource description.
	does string
	// Settings this kind has besides the shared ones, read back as returned by the API unless errorsIntegrationFromAPI
	// converts them.
	own map[string]*schema.Schema
}

var errorsIntegrationNotificationDescriptions = map[string]string{
	"spikes":                   "the application has a spike in errors.",
	"spike_protection":         "spike protection starts sampling exceptions.",
	"new_releases":             "a new release is observed.",
	"new_errors":               "a new error occurs.",
	"occurrence_milestones":    "an error reaches an occurrence milestone (10, 100, 200, 500, …).",
	"occurrence_threshold":     "an error occurs more than `occurrence_threshold_count` times per `occurrence_threshold_interval`.",
	"affected_users_threshold": "an error affects more than `affected_users_threshold_count` users.",
	"automatically_reopened":   "an error is automatically re-opened.",
}

func errorsIntegrationApplicationSchema(description string) *schema.Schema {
	return &schema.Schema{
		Description: description,
		Type:        schema.TypeSet,
		Optional:    true,
		Computed:    true,
		ConfigMode:  schema.SchemaConfigModeAttr,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"application_id": {
					Description: "The ID of the Errors application, e.g. `logtail_errors_application.this.id`.",
					Type:        schema.TypeInt,
					Required:    true,
				},
				"environments": {
					Description: "Environments of the application to include, e.g. `[\"production\"]`. Set to `[]` to include every environment.",
					Type:        schema.TypeList,
					Optional:    true,
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
			},
		},
	}
}

func errorsIntegrationSchema(kind errorsIntegrationKind) map[string]*schema.Schema {
	notificationSchema := map[string]*schema.Schema{}
	for _, key := range kind.notifications {
		notificationSchema[key] = &schema.Schema{
			Description: "Notify when " + errorsIntegrationNotificationDescriptions[key],
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
		}
	}

	s := map[string]*schema.Schema{
		"id": {
			Description: "The ID of the integration, the same as `integration_id`.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"integration_id": {
			Description: fmt.Sprintf("The ID of the existing %s integration to manage, as in the URL of its Configure page in Error tracking › Integrations, or as listed by `GET %s`. Changing it adopts a different integration and leaves the previous one as it is.", kind.product, kind.path),
			Type:        schema.TypeInt,
			Required:    true,
			ForceNew:    true,
		},
		"team_id": {
			Description: "The ID of the team the integration belongs to.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"team_name": {
			Description: "The name of the team the integration belongs to.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		"notifications": {
			Description: "The \"Notify me when\" toggles. Toggles left out keep their current value.",
			Type:        schema.TypeList,
			Optional:    true,
			Computed:    true,
			MaxItems:    1,
			Elem:        &schema.Resource{Schema: notificationSchema},
		},
		"occurrence_threshold_count": {
			Description:  "How many occurrences per `occurrence_threshold_interval` trigger the `occurrence_threshold` notification.",
			Type:         schema.TypeInt,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.IntAtLeast(1),
		},
		"occurrence_threshold_interval": {
			Description:  "The interval of `occurrence_threshold_count`: `minute`, `hour`, `day` or `week`.",
			Type:         schema.TypeString,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.StringInSlice([]string{"minute", "hour", "day", "week"}, false),
		},
		"affected_users_threshold_count": {
			Description:  "How many affected users trigger the `affected_users_threshold` notification.",
			Type:         schema.TypeInt,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.IntAtLeast(1),
		},
		"tag_filter": {
			Description: "Only errors whose tags match every row reach the integration, each row as `{ tag = \"region\", values = [\"eu\"] }`. Set to `[]` to remove the filter.",
			Type:        schema.TypeList,
			Optional:    true,
			Computed:    true,
			ConfigMode:  schema.SchemaConfigModeAttr,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"tag": {
						Description: "The tag name, e.g. `environment`.",
						Type:        schema.TypeString,
						Required:    true,
					},
					"values": {
						Description: "The values of the tag that match, e.g. `[\"production\"]`.",
						Type:        schema.TypeList,
						Required:    true,
						Elem:        &schema.Schema{Type: schema.TypeString},
					},
				},
			},
		},
	}
	if kind.applications {
		s["applications"] = errorsIntegrationApplicationSchema("The Errors applications whose errors reach the integration, each as `{ application_id = …, environments = […] }` with `environments = []` for every environment. Set to `[]` to include every application.")
	}
	for k, v := range kind.own {
		s[k] = v
	}
	return s
}

func newErrorsIntegrationResource(kind errorsIntegrationKind) *schema.Resource {
	return &schema.Resource{
		CreateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return errorsIntegrationCreate(ctx, d, meta, kind)
		},
		ReadContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return errorsIntegrationRead(ctx, d, meta, kind)
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return errorsIntegrationUpdate(ctx, d, meta, kind)
		},
		DeleteContext: errorsIntegrationDelete,
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
			return errorsIntegrationCustomizeDiff(d, kind)
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: fmt.Sprintf(
			"This resource manages the settings of an existing Error tracking %s integration: %s "+
				"Connecting an integration needs authorization in the Better Stack UI, so this resource doesn't create one. It adopts the integration given by `integration_id`, "+
				"applies the settings in your configuration and leaves every setting you don't configure as it is. "+
				"Destroying the resource only removes it from Terraform state: the integration stays connected with its current settings. "+
				"You can also import an integration by its ID.",
			kind.product, kind.does),
		Schema: errorsIntegrationSchema(kind),
	}
}

type errorsIntegrationHTTPResponse struct {
	Data struct {
		ID         string                 `json:"id"`
		Attributes map[string]interface{} `json:"attributes"`
	} `json:"data"`
}

func errorsIntegrationURL(kind errorsIntegrationKind, id string) string {
	return fmt.Sprintf("%s/%s", kind.path, id)
}

func errorsIntegrationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}, kind errorsIntegrationKind) diag.Diagnostics {
	id := strconv.Itoa(d.Get("integration_id").(int))
	// Adopting checks the integration exists before any setting changes, so a wrong ID fails with a clear error.
	var out errorsIntegrationHTTPResponse
	if derr, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), errorsIntegrationURL(kind, id), &out); derr != nil {
		return derr
	} else if !ok {
		return diag.Errorf("%s integration %s was not found. Connect it in Error tracking › Integrations first, then use the ID from its Configure page.", kind.product, id)
	}
	d.SetId(id)

	if derr := errorsIntegrationPatch(ctx, d, meta, kind, false); derr != nil {
		return derr
	}
	return errorsIntegrationRead(ctx, d, meta, kind)
}

func errorsIntegrationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}, kind errorsIntegrationKind) diag.Diagnostics {
	if derr := errorsIntegrationPatch(ctx, d, meta, kind, true); derr != nil {
		return derr
	}
	return errorsIntegrationRead(ctx, d, meta, kind)
}

// errorsIntegrationPatch sends the configured settings: on create every one set in the configuration, on update only the
// changed ones. The API changes only the settings it is sent, and only the toggles given within notifications.
func errorsIntegrationPatch(ctx context.Context, d *schema.ResourceData, meta interface{}, kind errorsIntegrationKind, onlyChanged bool) diag.Diagnostics {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() {
		return nil
	}
	req := map[string]interface{}{}
	for key, s := range errorsIntegrationSchema(kind) {
		if !s.Optional || key == "integration_id" {
			continue
		}
		v := raw.GetAttr(key)
		if v.IsNull() || !v.IsKnown() {
			continue
		}
		current := d.Get(key)
		if v.Type() == cty.String {
			// The SDK reads an empty configured string as unset, so the configured text is taken from the configuration.
			current = v.AsString()
			if old, _ := d.GetChange(key); onlyChanged && old.(string) == current {
				continue
			}
		} else if onlyChanged && !d.HasChange(key) {
			continue
		}
		value, err := errorsIntegrationToAPI(key, current, v)
		if err != nil {
			return diag.FromErr(err)
		}
		if toggles, ok := value.(map[string]interface{}); ok && key == "notifications" && len(toggles) == 0 {
			continue
		}
		req[key] = value
	}
	if len(req) == 0 {
		return nil
	}
	return resourceUpdateWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), errorsIntegrationURL(kind, d.Id()), req)
}

func errorsIntegrationToAPI(key string, value interface{}, raw cty.Value) (interface{}, error) {
	switch key {
	case "notifications":
		// Only the toggles written in the configuration, so the rest keep their value.
		toggles := map[string]interface{}{}
		list := value.([]interface{})
		if len(list) == 0 || list[0] == nil || raw.LengthInt() == 0 {
			return toggles, nil
		}
		planned := list[0].(map[string]interface{})
		block := raw.Index(cty.NumberIntVal(0))
		for name, v := range planned {
			if attr := block.GetAttr(name); !attr.IsNull() {
				toggles[name] = v
			}
		}
		return toggles, nil
	case "applications":
		return errorsIntegrationApplicationsToAPI(value.(*schema.Set).List()), nil
	case "routes":
		routes := []interface{}{}
		for _, r := range value.([]interface{}) {
			route := r.(map[string]interface{})
			routes = append(routes, map[string]interface{}{
				"applications": errorsIntegrationApplicationsToAPI(route["applications"].(*schema.Set).List()),
				"channels":     route["channel_ids"],
			})
		}
		return routes, nil
	case "linear_label_ids":
		ids := value.(*schema.Set).List()
		sort.Slice(ids, func(i, j int) bool { return ids[i].(string) < ids[j].(string) })
		return ids, nil
	case "jira_fields":
		var fields map[string]interface{}
		if err := json.Unmarshal([]byte(value.(string)), &fields); err != nil || fields == nil {
			return nil, fmt.Errorf("jira_fields must be a JSON object of Jira field IDs and their values, e.g. jsonencode({ customfield_10001 = \"value\" })")
		}
		return fields, nil
	default:
		return value, nil
	}
}

func errorsIntegrationApplicationsToAPI(list []interface{}) []interface{} {
	applications := []interface{}{}
	for _, a := range list {
		application := a.(map[string]interface{})
		environments := application["environments"]
		if environments == nil {
			environments = []interface{}{}
		}
		applications = append(applications, map[string]interface{}{
			"application_id": application["application_id"],
			"environments":   environments,
		})
	}
	sort.Slice(applications, func(i, j int) bool {
		return applications[i].(map[string]interface{})["application_id"].(int) < applications[j].(map[string]interface{})["application_id"].(int)
	})
	return applications
}

func errorsIntegrationRead(ctx context.Context, d *schema.ResourceData, meta interface{}, kind errorsIntegrationKind) diag.Diagnostics {
	var out errorsIntegrationHTTPResponse
	if derr, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).ErrorsBaseURL(), errorsIntegrationURL(kind, d.Id()), &out); derr != nil {
		return derr
	} else if !ok {
		d.SetId("") // Disconnected in the UI: drop it from state, so the next apply fails until it is connected again.
		return nil
	}

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.Errorf("the ID of a %s integration is a number, got %q", kind.product, d.Id())
	}
	var derr diag.Diagnostics
	if err := d.Set("integration_id", id); err != nil {
		derr = append(derr, diag.FromErr(err)...)
	}
	for key := range errorsIntegrationSchema(kind) {
		if key == "id" || key == "integration_id" {
			continue
		}
		value, ok := out.Data.Attributes[key]
		if !ok {
			continue
		}
		converted, err := errorsIntegrationFromAPI(key, value, d)
		if err != nil {
			derr = append(derr, diag.FromErr(err)...)
			continue
		}
		if err := d.Set(key, converted); err != nil {
			derr = append(derr, diag.FromErr(fmt.Errorf("%s: %w", key, err))...)
		}
	}
	return derr
}

func errorsIntegrationFromAPI(key string, value interface{}, d *schema.ResourceData) (interface{}, error) {
	switch key {
	case "notifications":
		return []interface{}{value}, nil
	case "applications":
		return errorsIntegrationApplicationsFromAPI(value), nil
	case "routes":
		routes := []interface{}{}
		for _, r := range asList(value) {
			route, _ := r.(map[string]interface{})
			channelIDs := []interface{}{}
			for _, c := range asList(route["channels"]) {
				if channel, ok := c.(map[string]interface{}); ok {
					channelIDs = append(channelIDs, channel["id"])
				}
			}
			routes = append(routes, map[string]interface{}{
				"applications": errorsIntegrationApplicationsFromAPI(route["applications"]),
				"channel_ids":  channelIDs,
			})
		}
		return routes, nil
	case "jira_fields":
		// Keep the configured text while it means the same JSON, so formatting never shows as a change.
		if current, ok := d.Get(key).(string); ok && current != "" && jsonEqual(current, value) {
			return current, nil
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return string(encoded), nil
	default:
		if f, ok := value.(float64); ok {
			return int(f), nil
		}
		return value, nil
	}
}

func errorsIntegrationApplicationsFromAPI(value interface{}) []interface{} {
	applications := []interface{}{}
	for _, a := range asList(value) {
		application, _ := a.(map[string]interface{})
		id, _ := application["application_id"].(float64)
		environments := asList(application["environments"])
		if environments == nil {
			environments = []interface{}{}
		}
		applications = append(applications, map[string]interface{}{
			"application_id": int(id),
			"environments":   environments,
		})
	}
	return applications
}

func asList(value interface{}) []interface{} {
	list, _ := value.([]interface{})
	return list
}

func jsonEqual(text string, value interface{}) bool {
	var parsed interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return false
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return false
	}
	var normalized interface{}
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return false
	}
	return reflect.DeepEqual(parsed, normalized)
}

// Never calls the API: an integration is disconnected in the Better Stack UI, so removing the resource leaves it connected.
func errorsIntegrationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

// errorsIntegrationCustomizeDiff plans an explicitly empty value (`applications = []`, `tag_filter = []`, `name = ""`)
// as a change: for Optional+Computed attributes the SDK otherwise treats an empty value as unset and keeps the state.
func errorsIntegrationCustomizeDiff(d *schema.ResourceDiff, kind errorsIntegrationKind) error {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || d.Id() == "" {
		return nil
	}
	for key, s := range errorsIntegrationSchema(kind) {
		if !s.Optional || !s.Computed {
			continue
		}
		v := raw.GetAttr(key)
		if v.IsNull() || !v.IsWhollyKnown() {
			continue
		}
		empty := false
		switch {
		case v.Type() == cty.String:
			empty = v.AsString() == ""
		case v.Type().IsListType() || v.Type().IsSetType() || v.Type().IsTupleType():
			empty = v.LengthInt() == 0
		}
		if !empty || key == "notifications" {
			continue
		}
		old, _ := d.GetChange(key)
		if isEmptyValue(old) {
			continue
		}
		switch s.Type {
		case schema.TypeString:
			// The SDK drops an empty string set as the planned value, so the change is planned as known after apply;
			// the update sends the configured "" and reads it back.
			if err := d.SetNewComputed(key); err != nil {
				return err
			}
		default:
			if err := d.SetNew(key, []interface{}{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func isEmptyValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []interface{}:
		return len(x) == 0
	case *schema.Set:
		return x.Len() == 0
	}
	return false
}
