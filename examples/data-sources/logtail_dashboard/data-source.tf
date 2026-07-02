data "logtail_dashboard" "production" {
  id = logtail_dashboard.production.id
}

# Build a console link from the looked-up team and dashboard IDs
output "existing_dashboard_url" {
  value = "https://telemetry.betterstack.com/team/${data.logtail_dashboard.production.team_id}/dashboards/${data.logtail_dashboard.production.id}"
}
