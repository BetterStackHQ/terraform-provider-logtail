data "logtail_dashboard_alert" "high_error_rate" {
  dashboard_id = logtail_dashboard.production.id
  chart_id     = logtail_dashboard_chart.request_rate.id
  name         = logtail_dashboard_alert.high_error_rate.name
}
