data "logtail_dashboard_section" "performance" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Performance"
}
