resource "logtail_source" "this" {
  name     = "Production logs"
  platform = "http"
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
