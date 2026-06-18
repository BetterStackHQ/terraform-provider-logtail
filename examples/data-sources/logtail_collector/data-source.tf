data "logtail_collector" "production" {
  name = "Production Docker"

  depends_on = [logtail_collector.production]
}
