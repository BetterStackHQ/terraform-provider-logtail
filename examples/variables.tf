variable "logtail_api_token" {
  type        = string
  description = <<EOF
Better Stack Telemetry API token
(https://betterstack.com/docs/logs/api/getting-started/#get-an-logs-api-token)
EOF
  # The value can be omitted if the LOGTAIL_API_TOKEN env var is set.
  default = null
}

# Credentials for the custom S3-compatible bucket example (logtail_source.custom_bucket).
# Only needed when applying that example - Better Stack validates them against the live
# bucket during creation. Keep the values out of version control, e.g. set them via
# TF_VAR_source_custom_bucket_* environment variables (CI takes them from GitHub secrets).
variable "source_custom_bucket_endpoint" {
  type        = string
  description = "S3-compatible bucket endpoint including the bucket name, e.g. https://s3.us-east-1.amazonaws.com/my-bucket"
  default     = null
}

variable "source_custom_bucket_access_key_id" {
  type        = string
  description = "Access key ID for the custom bucket"
  default     = null
}

# `sensitive = true` would be appropriate here, but the examples stay compatible
# with Terraform 0.13, which predates it.
variable "source_custom_bucket_secret_access_key" {
  type        = string
  description = "Secret access key for the custom bucket"
  default     = null
}
