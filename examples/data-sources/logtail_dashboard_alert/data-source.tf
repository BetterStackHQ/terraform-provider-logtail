data "logtail_dashboard_alert" "high_error_rate" {
  dashboard_id = logtail_dashboard.production.id
  chart_id     = logtail_dashboard_chart.request_rate.id
  name         = "High Error Rate"

  depends_on = [logtail_dashboard_alert.high_error_rate]
}

output "existing_dashboard_alert_type" {
  value = data.logtail_dashboard_alert.high_error_rate.alert_type
}
