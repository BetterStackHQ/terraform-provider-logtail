provider "logtail" {
  api_token = var.logtail_api_token
}

variable "logtail_api_token" {
  type    = string
  default = null
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
}
