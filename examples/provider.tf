provider "logtail" {
  # `api_token` can be omitted if the LOGTAIL_API_TOKEN env var is set.
  api_token = var.logtail_api_token
}
