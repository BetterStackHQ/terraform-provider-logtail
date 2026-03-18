data "logtail_dashboard_alert" "high_error_rate" {
  dashboard_id = "123"
  chart_id     = "456"
  name         = "High Error Rate"
}
