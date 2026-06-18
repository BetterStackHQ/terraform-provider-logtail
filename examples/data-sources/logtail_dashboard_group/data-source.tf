data "logtail_dashboard_group" "production" {
  name = "Production Dashboards"

  depends_on = [logtail_dashboard_group.production]
}
