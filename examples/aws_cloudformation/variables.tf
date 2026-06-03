variable "logtail_api_token" {
  type        = string
  description = <<EOF
Better Stack Telemetry API token
(https://betterstack.com/docs/logs/api/getting-started/#get-an-logs-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}

variable "connect_aws_account" {
  type        = bool
  description = <<EOF
Whether to connect the AWS account to the source by pasting the CloudFormation
role ARN / external ID back into Better Stack.

Leave this `false` on the FIRST `terraform apply` (the CloudFormation stack does
not exist yet, so its outputs are unknown — wiring them in immediately creates a
dependency cycle: the stack needs the source token, and the source would need the
stack's role ARN). After the first apply has created the source and the stack,
set it to `true` and apply again to connect the account. This mirrors the two
steps of the AWS CloudFormation setup in the Better Stack UI.
EOF
  default     = false
}
