variable "logtail_api_token" {
  type        = string
  description = <<EOF
Logtail API Token
(https://docs.logtail.com/api/getting-started#obtaining-an-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}
