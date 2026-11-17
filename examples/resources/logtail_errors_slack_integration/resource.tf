# Connect Slack in Error tracking › Integrations first, then adopt the integration by the ID
# from its Configure page. Settings left out stay as they are in Better Stack, and removing
# the resource leaves the integration connected.
resource "logtail_errors_slack_integration" "errors" {
  integration_id = 123456

  applications = [
    { application_id = logtail_errors_application.this.id, environments = ["production"] },
  ]
  post_updates = "thread_based"

  notifications {
    new_errors           = true
    occurrence_threshold = true
  }
  occurrence_threshold_count    = 100
  occurrence_threshold_interval = "hour"
}
