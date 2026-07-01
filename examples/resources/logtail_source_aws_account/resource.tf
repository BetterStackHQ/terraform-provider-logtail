# Link a source to an AWS account you've already connected, by account ID
# aws_account_id is write-only - the API never returns it, so it isn't refreshed
resource "logtail_source" "aws_existing" {
  name     = "AWS staging"
  platform = "aws"
}

resource "logtail_source_aws_account" "existing" {
  source_id      = logtail_source.aws_existing.id
  aws_account_id = "123456789012"
}

# Full setup in one terraform apply, creating the source and the IAM role
# a CloudFormation stack provisions the role and the link reads its outputs
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
