---
page_title: "Connecting an AWS account to a source"
subcategory: ""
description: |-
  Create an AWS source, provision the Better Stack IAM role with CloudFormation, and link the account in a single terraform apply.
---

# Connecting an AWS account to a source

An `aws` platform `logtail_source` ingests logs and metrics as soon as data is forwarded to its token. Enumerating CloudWatch log groups and enriching resources additionally needs a linked AWS account (an STS-validated IAM role). Until an account is linked, Better Stack shows the "Connect your AWS account" step and no role ARN, even while ingestion works.

You can do the whole flow (create the source, provision the IAM role with CloudFormation, and link the account) in a single `terraform apply`.

## Single apply with CloudFormation

Use a separate `logtail_source_aws_account` resource to paste the CloudFormation outputs back to the source. This keeps the dependency graph acyclic (`logtail_source` then `aws_cloudformation_stack` then `logtail_source_aws_account`), so it all applies in one run:

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

Do not set `aws_role_arn` / `aws_external_id` on the `logtail_source` when the ARN comes from an `aws_cloudformation_stack` in the same configuration: the stack consumes the source token and the source would consume the stack output, which is a dependency cycle. The separate `logtail_source_aws_account` resource breaks it.

## Inline, when the ARN comes from a variable

If the role ARN is supplied out-of-band (a variable, or a stack applied separately), set it directly on the source. There is no cycle because the ARN is a static input, and no extra resource is needed:

```terraform
resource "logtail_source" "aws" {
  name            = "AWS production"
  platform        = "aws"
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
