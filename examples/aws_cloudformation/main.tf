provider "logtail" {
  api_token = var.logtail_api_token
}

provider "aws" {
  # Configure your AWS region / credentials as usual.
  region = "us-east-1"
}

# Step 1 — the Better Stack source shell.
# Created first so its `token` / `id` / `data_region` (the CloudFormation
# ClusterId) are available to the stack below. On the first apply the AWS account
# is not connected yet — `aws_role_arn` / `aws_external_id` are only wired in once
# `var.connect_aws_account` is set to true (see variables.tf).
resource "logtail_source" "aws" {
  name     = "AWS production"
  platform = "aws"

  aws_role_arn    = var.connect_aws_account ? aws_cloudformation_stack.better_stack.outputs["IntegrationRoleArn"] : null
  aws_external_id = var.connect_aws_account ? aws_cloudformation_stack.better_stack.outputs["ExternalId"] : null
}

# Step 2 — deploy the Better Stack CloudFormation stack.
# It configures logs + metrics forwarding via AWS Firehose and creates the IAM
# role Better Stack assumes to enumerate resources and manage CloudWatch log-group
# subscriptions. The stack consumes the source's token / id / cluster name.
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

# Step 3 — connect the account.
# The stack's IntegrationRoleArn / ExternalId outputs are fed back into
# logtail_source.aws above. Run:
#
#   terraform apply                              # creates the source + stack
#   terraform apply -var connect_aws_account=true # connects the AWS account
#
# After the second apply the role-based resource enumeration and CloudWatch
# log-group subscription management are live — closing the loop that previously
# required a manual paste-back in the Better Stack UI.
