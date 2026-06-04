# AWS CloudFormation source

Provision a Better Stack **AWS CloudFormation** source end-to-end in a **single
`terraform apply`**: create the source, deploy the Better Stack CloudFormation stack in
your AWS account, and connect the AWS account by feeding the stack's role ARN / external
ID back into the source. This closes the loop that previously required a manual paste-back
in the Better Stack UI.

## The full flow

1. **Create the source.** `logtail_source` with `platform = "aws"` creates the source and
   exports `token`, `ingesting_host`, `id`, and `data_region` (the cluster name).
2. **Deploy the CloudFormation stack** in AWS using those values
   (`SourceToken` / `SourceId` / `ClusterId`). It sets up Firehose log + metric
   forwarding and creates the IAM role Better Stack assumes to enumerate resources and
   manage CloudWatch log-group subscriptions. The stack outputs `IntegrationRoleArn`
   and `ExternalId`.
3. **Connect the account** with a separate `logtail_source_aws_account` resource that
   references both the source id and the stack outputs. Create/update runs the exact same
   account-connect manager the UI uses (`PATCH /api/v1/sources/:source_id`).

## Why a separate `logtail_source_aws_account` resource

Putting `aws_role_arn` / `aws_external_id` directly on `logtail_source` and wiring them to
the stack output is a genuine dependency cycle: the stack needs the source's `token`, and
the source would then need the stack's `IntegrationRoleArn`. Splitting the connect step
into its own resource breaks the cycle — Terraform's dependency graph orders
`logtail_source_aws_account` **after** both the source and the stack, so everything
provisions in one pass:

```bash
terraform apply   # creates the source, deploys the stack, then connects the AWS account
```

No `connect_aws_account` toggle, no second apply.

## Semantics

- `source_id` is `ForceNew` — pointing the linkage at a different source recreates it and
  re-runs the connect against the new source.
- `aws_role_arn` / `aws_external_id` (or `aws_account_id` to reuse an already-connected
  account) update **in place**: rotating credentials re-PATCHes the source, it does not
  recreate anything. The credentials are write-only — the API never returns them, so they
  aren't refreshed from state.
- Destroying `logtail_source_aws_account` only removes it from Terraform state. The Sources
  API has no disconnect endpoint, so the AWS account stays linked to the source until the
  source itself is destroyed.

## Files

- `main.tf` — the source, the CloudFormation stack, and the `logtail_source_aws_account` connect step.
- `variables.tf` — `logtail_api_token`.
- `outputs.tf` — source id/token and the stack's `IntegrationRoleArn`.
- `versions.tf` — provider requirements.
