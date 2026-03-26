variable "logtail_api_token" {
  type        = string
  description = <<EOF
Better Stack Telemetry API token
(https://betterstack.com/docs/logs/api/getting-started/#get-an-logs-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}

variable "db_password" {
  description = "Database password for collector monitoring"
  type        = string
  # Default for e2e testing — the database host is not real, so the password is not used.
  default = "e2e_test_password"
}
