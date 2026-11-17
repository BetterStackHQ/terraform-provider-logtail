# Add the Better Stack app in Error tracking › Integrations › Microsoft Teams first, then adopt
# the integration by the ID from its Configure page. Settings left out stay as they are in
# Better Stack, and removing the resource leaves the integration connected.
resource "logtail_errors_microsoft_teams_integration" "errors" {
  integration_id = 123456

  # Without routes, every error goes to the channel the Better Stack app was added to.
  routes = [
    {
      applications = [
        { application_id = logtail_errors_application.this.id, environments = [] },
      ]
      channel_ids = ["19:0123456789abcdef@thread.tacv2"]
    },
  ]

  notifications {
    spikes = true
  }
}
