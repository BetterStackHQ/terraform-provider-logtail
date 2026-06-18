data "logtail_dashboard_chart" "request_rate" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Request Rate"

  depends_on = [logtail_dashboard_chart.request_rate]
}
