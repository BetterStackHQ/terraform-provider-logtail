---
page_title: "Connecting an AWS account to a source"
subcategory: ""
description: |-
  Create an AWS source, provision the Better Stack IAM role with CloudFormation, and link the account in a single terraform apply.
---

# Connecting an AWS account to a source

An `aws` platform `logtail_source` ingests logs and metrics as soon as data is forwarded to its token. Enumerating CloudWatch log groups and enriching resources additionally needs a linked AWS account (an STS-validated IAM role). Until an account is linked, Better Stack shows the "Connect your AWS account" step and no role ARN, even while ingestion works.

Link the account with a `logtail_source_aws_account` resource. The role ARN and external ID live on that resource rather than on `logtail_source` itself: the CloudFormation stack consumes the source token and the link consumes the stack output, so keeping them separate keeps the dependency graph acyclic and lets the whole flow apply in one run.

## Single apply with CloudFormation

Create the source, let a CloudFormation stack provision the IAM role, then link the account from its outputs (`logtail_source` then `aws_cloudformation_stack` then `logtail_source_aws_account`):

```terraform
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
```

## When the ARN comes from a variable

If the role ARN is supplied out-of-band (a variable, or a stack applied separately), pass it to the link resource directly. There is no cycle because the ARN is a static input:

```terraform
resource "logtail_source" "aws" {
  name     = "AWS production"
  platform = "aws"
}

resource "logtail_source_aws_account" "aws" {
  source_id       = logtail_source.aws.id
  aws_role_arn    = var.betterstack_role_arn
  aws_external_id = var.betterstack_external_id
}
```

## Reusing an already-connected account

To attach a source to an account you have already connected, reference it by ID:

```terraform
resource "logtail_source_aws_account" "aws" {
  source_id      = logtail_source.aws.id
  aws_account_id = "123456789012"
}
```
