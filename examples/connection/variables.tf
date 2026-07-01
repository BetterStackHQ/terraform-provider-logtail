variable "logtail_api_token" {
  type        = string
  description = <<EOF
Better Stack Global API token
(https://betterstack.com/docs/logs/api/getting-started/#get-a-global-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}

variable "logtail_team_name" {
  type        = string
  description = "The Better Stack team to create the source and connection in."
  default     = "Terraform E2E Tests"
}
