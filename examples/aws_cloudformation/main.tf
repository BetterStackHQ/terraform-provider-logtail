provider "logtail" {
  api_token = var.logtail_api_token
}

provider "aws" {
  # Configure your AWS region / credentials as usual.
  region = "us-east-1"
}

# Step 1 — the Better Stack source shell.
# Exports `token` / `id` / `data_region` (the CloudFormation ClusterId) for the stack below.
resource "logtail_source" "aws" {
  name     = "AWS production"
  platform = "aws"
}

# Step 2 — deploy the Better Stack CloudFormation stack.
# It configures logs + metrics forwarding via AWS Firehose and creates the IAM role
# Better Stack assumes to enumerate resources and manage CloudWatch log-group
# subscriptions. The stack consumes the source's token / id / cluster name and outputs
# the IntegrationRoleArn / ExternalId used to connect the account in step 3.
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

# Step 3 — connect the AWS account.
# A separate resource so Terraform's dependency graph orders it AFTER both the source and
# the stack: it references the source id and the stack's outputs, so a single
# `terraform apply` creates the source, deploys the stack, then PATCHes the role ARN /
# external ID onto the source (the same account-connect the Better Stack UI runs).
resource "logtail_source_aws_account" "aws" {
  source_id       = logtail_source.aws.id
  aws_role_arn    = aws_cloudformation_stack.better_stack.outputs["IntegrationRoleArn"]
  aws_external_id = aws_cloudformation_stack.better_stack.outputs["ExternalId"]
}
