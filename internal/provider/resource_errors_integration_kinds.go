package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var errorsIntegrationAlertNotifications = []string{
	"spikes", "spike_protection", "new_releases", "new_errors",
	"occurrence_milestones", "occurrence_threshold", "affected_users_threshold", "automatically_reopened",
}

var errorsIntegrationIssueNotifications = []string{
	"spikes", "new_errors", "automatically_reopened",
	"occurrence_milestones", "occurrence_threshold", "affected_users_threshold",
}

func computedString(description string) *schema.Schema {
	return &schema.Schema{Description: description, Type: schema.TypeString, Computed: true}
}

func settingString(description string) *schema.Schema {
	return &schema.Schema{Description: description, Type: schema.TypeString, Optional: true, Computed: true}
}

func settingBool(description string) *schema.Schema {
	return &schema.Schema{Description: description, Type: schema.TypeBool, Optional: true, Computed: true}
}

var errorsSlackIntegrationKind = errorsIntegrationKind{
	resourceName:  "logtail_errors_slack_integration",
	path:          "/api/v1/slack-integrations",
	product:       "Slack",
	notifications: errorsIntegrationAlertNotifications,
	applications:  true,
	does:          "which errors it posts to its Slack channel, and how.",
	own: map[string]*schema.Schema{
		"slack_team_name":    computedString("The Slack workspace the integration posts to."),
		"slack_channel_name": computedString("The Slack channel the integration posts to."),
		"post_updates": {
			Description:  "How updates are posted: `single_channel` posts every update to the channel directly, `thread_based` posts new errors to the channel and their updates in a thread.",
			Type:         schema.TypeString,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.StringInSlice([]string{"single_channel", "thread_based"}, false),
		},
		"broadcast_updates": settingBool("With `post_updates = \"thread_based\"`, whether important updates (resolved and re-opened errors, spikes and occurrence milestones) are also sent to the channel."),
		"ai_sre_on_mention": settingBool("Whether tagging Better Stack in the channel starts an AI SRE conversation."),
	},
}

var errorsMicrosoftTeamsIntegrationKind = errorsIntegrationKind{
	resourceName:  "logtail_errors_microsoft_teams_integration",
	path:          "/api/v1/microsoft-teams-integrations",
	product:       "Microsoft Teams",
	notifications: errorsIntegrationAlertNotifications,
	applications:  false,
	does:          "which errors it posts, and to which channels.",
	own: map[string]*schema.Schema{
		"microsoft_teams_team_name":    computedString("The Microsoft Teams team the Better Stack app was added to."),
		"microsoft_teams_channel_name": computedString("The channel the Better Stack app was added to. Without `routes`, every error goes to this channel."),
		"routes": {
			Description: "Which applications' errors go to which channels, each route as `{ applications = […], channel_ids = […] }`. Without routes, every error goes to the channel the Better Stack app was added to. Set to `[]` to remove every route.",
			Type:        schema.TypeList,
			Optional:    true,
			Computed:    true,
			ConfigMode:  schema.SchemaConfigModeAttr,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"applications": func() *schema.Schema {
						s := errorsIntegrationApplicationSchema("The Errors applications whose errors this route sends, each as `{ application_id = …, environments = […] }` with `environments = []` for every environment.")
						s.Computed = false
						return s
					}(),
					"channel_ids": {
						Description: "IDs of standard channels in the team the Better Stack app was added to.",
						Type:        schema.TypeList,
						Required:    true,
						MinItems:    1,
						Elem:        &schema.Schema{Type: schema.TypeString},
					},
				},
			},
		},
	},
}

var errorsLinearIntegrationKind = errorsIntegrationKind{
	resourceName:  "logtail_errors_linear_integration",
	path:          "/api/v1/linear-integrations",
	product:       "Linear",
	notifications: errorsIntegrationIssueNotifications,
	applications:  true,
	does:          "which errors it files as Linear issues, and with which properties.",
	own: map[string]*schema.Schema{
		"name":                      settingString("The name of the integration in Better Stack."),
		"linear_team_id":            settingString("The ID of the Linear team issues are filed in. Changing it clears the statuses, project and labels of the previous team unless you set them too."),
		"linear_assignee_id":        settingString("The ID of the Linear user new issues are assigned to. Set to `\"\"` to leave issues unassigned."),
		"linear_status_id":          settingString("The ID of the Linear workflow state new issues start in. Set to `\"\"` for the team's default."),
		"linear_resolved_status_id": settingString("The ID of the Linear workflow state issues move to when their error is resolved."),
		"linear_project_id":         settingString("The ID of the Linear project new issues are added to. Set to `\"\"` for none."),
		"linear_label_ids": {
			Description: "IDs of the Linear labels added to new issues. Set to `[]` for none.",
			Type:        schema.TypeSet,
			Optional:    true,
			Computed:    true,
			ConfigMode:  schema.SchemaConfigModeAttr,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		"linear_priority": {
			Description:  "The priority of new issues: `no_priority`, `urgent`, `high`, `medium` or `low`.",
			Type:         schema.TypeString,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.StringInSlice([]string{"no_priority", "urgent", "high", "medium", "low"}, false),
		},
		"auto_reopen":   settingBool("Whether a resolved error that occurs again reopens its Linear issue."),
		"auto_resolve":  settingBool("Whether completing the Linear issue resolves its error."),
		"sync_comments": settingBool("Whether comments are synced between the error and its Linear issue."),
	},
}

var errorsJiraIntegrationKind = errorsIntegrationKind{
	resourceName:  "logtail_errors_jira_integration",
	path:          "/api/v1/jira-integrations",
	product:       "Jira",
	notifications: errorsIntegrationIssueNotifications,
	applications:  true,
	does:          "which errors it files as Jira work items, and with which properties.",
	own: map[string]*schema.Schema{
		"name":          settingString("The name of the integration in Better Stack."),
		"jira_site_url": computedString("The Jira site the integration files work items on."),
		"jira_project_key": settingString(
			"The key of the Jira space (project) work items are filed in. Changing it needs `jira_issue_type_id` too, and clears the previous space's `jira_fields` unless you set them."),
		"jira_issue_type_id": settingString("The ID of the Jira work type of new work items."),
		"jira_fields": {
			Description:  "Values of further Jira fields of new work items, as a JSON object of Jira field IDs and their values, e.g. `jsonencode({ customfield_10001 = \"value\" })`.",
			Type:         schema.TypeString,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.StringIsJSON,
			DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
				return old != "" && new != "" && jsonTextEqual(old, new)
			},
		},
	},
}

func jsonTextEqual(a, b string) bool {
	var parsed interface{}
	if err := json.Unmarshal([]byte(b), &parsed); err != nil {
		return false
	}
	return jsonEqual(a, parsed)
}

func newErrorsSlackIntegrationResource() *schema.Resource {
	return newErrorsIntegrationResource(errorsSlackIntegrationKind)
}

func newErrorsMicrosoftTeamsIntegrationResource() *schema.Resource {
	return newErrorsIntegrationResource(errorsMicrosoftTeamsIntegrationKind)
}

func newErrorsLinearIntegrationResource() *schema.Resource {
	return newErrorsIntegrationResource(errorsLinearIntegrationKind)
}

func newErrorsJiraIntegrationResource() *schema.Resource {
	return newErrorsIntegrationResource(errorsJiraIntegrationKind)
}
