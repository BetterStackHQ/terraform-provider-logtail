# Connect Linear in Error tracking › Integrations first, then adopt the integration by the ID
# from its Configure page. Settings left out stay as they are in Better Stack, and removing
# the resource leaves the integration connected.
resource "logtail_errors_linear_integration" "errors" {
  integration_id = 123456

  applications = [
    { application_id = logtail_errors_application.this.id, environments = ["production"] },
  ]
  tag_filter = [
    { tag = "region", values = ["eu"] },
  ]

  # File an issue for every new error, as urgent, and resolve the error with its issue.
  notifications {
    new_errors = true
  }
  linear_priority = "urgent"
  auto_resolve    = true
}
