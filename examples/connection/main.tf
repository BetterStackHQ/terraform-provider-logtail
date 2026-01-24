provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "my_source" {
  platform    = "http"
  team_name   = "Terraform E2E Tests"
  data_region = "us_east"
  name        = "Terraform source for connection"
}

resource "logtail_connection" "example" {
  client_type = "clickhouse"
  team_names  = [logtail_source.my_source.team_name]
  data_region = logtail_source.my_source.data_region
  note        = "Example connection from Terraform"
}
