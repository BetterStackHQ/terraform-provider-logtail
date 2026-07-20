resource "logtail_dashboard_section" "performance" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Performance"
  y            = 8
  explanation  = "Latency and throughput of the public API"
}

resource "logtail_dashboard_section" "errors" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Errors"
  y            = 16
  collapsed    = true
}
