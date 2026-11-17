# Connect Jira in Error tracking › Integrations first, then adopt the integration by the ID
# from its Configure page. Settings left out stay as they are in Better Stack, and removing
# the resource leaves the integration connected.
resource "logtail_errors_jira_integration" "errors" {
  integration_id = 123456

  applications = [
    { application_id = logtail_errors_application.this.id, environments = ["production"] },
  ]

  # File a work item when an error affects more than 50 users.
  notifications {
    affected_users_threshold = true
  }
  affected_users_threshold_count = 50

  jira_project_key   = "OPS"
  jira_issue_type_id = "10001"
  jira_fields        = jsonencode({ labels = ["errors"] })
}
