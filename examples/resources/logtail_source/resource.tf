resource "logtail_source" "this" {
  name     = "Production"
  platform = "http"

  # Server-side VRL transformations are applied per telemetry type during ingestion.
  # vrl_transformation_logs runs on logs; vrl_transformation_spans runs on traces.
  vrl_transformation_logs = <<-EOT
    .level = downcase(to_string!(.level))
  EOT

  vrl_transformation_spans = <<-EOT
    # Drop noisy health-check spans
    if .name == "GET /api/health" {
        del(.)
    }
  EOT

  # Metric names to drop as spam: rejected during ingestion, never billed.
  blocked_metrics = ["go_gc_duration_seconds", "go_memstats_heap_idle_bytes"]
}

# A source that scrapes Prometheus metrics endpoints on a schedule.
resource "logtail_source" "scrape" {
  name                  = "Prometheus scrape"
  platform              = "prometheus_scrape"
  scrape_urls           = ["https://myserver.example.com/metrics"]
  scrape_frequency_secs = 30
  scrape_request_headers = [
    {
      name  = "User-Agent"
      value = "My Scraper"
    }
  ]
  scrape_request_basic_auth_user     = "foo"
  scrape_request_basic_auth_password = "bar"
  skip_ssl_verify                    = true
}
