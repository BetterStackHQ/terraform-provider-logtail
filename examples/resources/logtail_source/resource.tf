# Minimal HTTP source config
resource "logtail_source" "this" {
  name     = "Production"
  platform = "http"
}

# Server-side VRL transforms run per telemetry type during ingestion
# blocked_metrics drops spam metrics before they are billed
resource "logtail_source" "transformed" {
  name     = "Production (transformed)"
  platform = "http"

  vrl_transformation_logs = <<-EOT
    .level = downcase(to_string!(.level))
  EOT

  vrl_transformation_spans = <<-EOT
    # Drop noisy health-check spans
    if .name == "GET /api/health" {
        del(.)
    }
  EOT

  blocked_metrics = ["go_gc_duration_seconds", "go_memstats_heap_idle_bytes"]
}

# Scrape Prometheus metrics endpoints on a schedule
resource "logtail_source" "scrape" {
  name                  = "Prometheus scrape"
  platform              = "prometheus_scrape"
  scrape_urls           = ["https://myserver.example.com/metrics"]
  scrape_frequency_secs = 30
  scrape_request_headers = [
    {
      name  = "User-Agent"
      value = "My Scraper"
    }
  ]
  scrape_request_basic_auth_user     = "foo"
  scrape_request_basic_auth_password = "bar"
  skip_ssl_verify                    = true
}

# Pin the region, set retention and file the source under a group
resource "logtail_source" "configured" {
  name              = "Production (EU)"
  platform          = "http"
  data_region       = "germany"
  logs_retention    = 60
  metrics_retention = 90
  source_group_id   = logtail_source_group.this.id

  # Created paused, ingestion will not start unless you flip this
  ingesting_paused = true

  # Format Live tail output with columns wrapped in {braces}
  live_tail_pattern = "{level} {message}"
}

# Connect an AWS account in a single terraform apply: create the source, let a
# CloudFormation stack provision the IAM role, then link the account from its outputs.
# Set aws_role_arn/aws_external_id on logtail_source below only when the ARN comes from
# elsewhere (a variable / out-of-band stack) - injecting a stack output back into this
# same source would form a dependency cycle, so use logtail_source_aws_account for that.
resource "logtail_source" "aws" {
  name     = "AWS production"
  platform = "aws"
}

resource "aws_cloudformation_stack" "better_stack" {
  name         = "better-stack-integration"
  template_url = "https://better-stack-cloudformation.s3.amazonaws.com/better-stack-full.yaml"
  capabilities = ["CAPABILITY_NAMED_IAM"]

  parameters = {
    ClusterId   = logtail_source.aws.data_region # reads back as cloud_cluster.name
    SourceToken = logtail_source.aws.token
    SourceId    = logtail_source.aws.id
  }
}

resource "logtail_source_aws_account" "aws" {
  source_id       = logtail_source.aws.id
  aws_role_arn    = aws_cloudformation_stack.better_stack.outputs["IntegrationRoleArn"]
  aws_external_id = aws_cloudformation_stack.better_stack.outputs["ExternalId"]
}

# When the role ARN is supplied out-of-band (e.g. a variable), connect inline on the
# source itself - no separate resource needed, and no cycle since the ARN is a static input.
resource "logtail_source" "aws_inline" {
  name            = "AWS production (from variable)"
  platform        = "aws"
  aws_role_arn    = var.betterstack_role_arn
  aws_external_id = var.betterstack_external_id
}
