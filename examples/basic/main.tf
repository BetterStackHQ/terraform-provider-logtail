provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "this" {
  name     = "Terraform Basic Source"
  platform = "http"
}

resource "logtail_errors_application" "this" {
  name     = "Terraform Basic Errors Application"
  platform = "ruby_errors"
}
