data "logtail_dashboard_chart" "request_rate" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Request Rate"

  # The chart is created in the same configuration, so read it after that.
  depends_on = [logtail_dashboard_chart.request_rate]
}

output "existing_chart_type" {
  value = data.logtail_dashboard_chart.request_rate.chart_type
}
