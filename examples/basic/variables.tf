variable "logtail_api_token" {
  type        = string
  description = <<EOF
Better Stack Logs API Token
(https://betterstack.com/docs/logs/api/getting-started/#get-an-logs-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}
