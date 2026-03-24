resource "logtail_dashboard_alert" "high_error_rate" {
  dashboard_id        = logtail_dashboard.production.id
  chart_id            = logtail_dashboard_chart.request_rate.id
  name                = "High Error Rate"
  alert_type          = "threshold"
  operator            = "higher_than"
  value               = 100
  query_period        = 300
  confirmation_period = 60

  email = true
  push  = true
}
