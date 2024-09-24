provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "this" {
  name                  = "Terraform Scrape Source"
  platform              = "prometheus_scrape"
  scrape_urls           = "https://myserver.example.com/metrics"
  scrape_frequency_secs = 30
  scrape_request_headers = [
    {
      name  = "User-Agent"
      value = "My Scraper"
    }
  ]
  scrape_request_basic_auth_user     = "foo"
  scrape_request_basic_auth_password = "bah"
}
