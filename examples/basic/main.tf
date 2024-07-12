provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "this" {
  name     = "Terraform Basic Source"
  platform = "http"
}
