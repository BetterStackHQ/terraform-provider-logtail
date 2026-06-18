data "logtail_dashboard" "production" {
  name = "Production overview"

  depends_on = [logtail_dashboard.production]
}
