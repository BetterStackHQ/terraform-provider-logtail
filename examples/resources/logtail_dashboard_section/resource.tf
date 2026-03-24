resource "logtail_dashboard_section" "performance" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Performance"
  y            = 8
}

resource "logtail_dashboard_section" "errors" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Errors"
  y            = 16
  collapsed    = true
}
