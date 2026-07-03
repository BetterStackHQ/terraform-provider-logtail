# Minimal HTTP source config
resource "logtail_source" "this" {
  name     = "Production"
  platform = "http"
}

# Server-side VRL transforms run per telemetry type during ingestion
# blocked_metrics drops spam metrics before they are billed
resource "logtail_source" "transformed" {
  name     = "Production (transformed)"
  platform = "http"

  vrl_transformation_logs = <<-EOT
    .level = downcase(to_string!(.level))
  EOT

  vrl_transformation_spans = <<-EOT
    # Drop noisy health-check spans
    if .name == "GET /api/health" {
        del(.)
    }
  EOT

  blocked_metrics = ["go_gc_duration_seconds", "go_memstats_heap_idle_bytes"]
}

# Scrape Prometheus metrics endpoints on a schedule
resource "logtail_source" "scrape" {
  name                  = "Prometheus scrape"
  platform              = "prometheus_scrape"
  source_group_id       = logtail_source_group.secondary.id
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

# Pin the region, set retention and file the source under a group
resource "logtail_source" "configured" {
  name              = "Production (EU)"
  platform          = "http"
  data_region       = "germany"
  logs_retention    = 60
  metrics_retention = 90
  source_group_id   = logtail_source_group.this.id

  # Created paused, ingestion will not start unless you flip this
  ingesting_paused = true

  # Format Live tail output with columns wrapped in {braces}
  live_tail_pattern = "{level} {message}"
}
