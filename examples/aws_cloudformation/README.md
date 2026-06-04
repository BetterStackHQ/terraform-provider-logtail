# AWS CloudFormation source

Provision a Better Stack **AWS CloudFormation** source end-to-end: create the source,
deploy the Better Stack CloudFormation stack in your AWS account, and connect the AWS
account by feeding the stack's role ARN / external ID back into the source. This closes
the loop that previously required a manual paste-back in the Better Stack UI.

## The full flow

1. **Create the source.** `logtail_source` with `platform = "aws"` creates the source and
   exports `token`, `ingesting_host`, `id`, and `data_region` (the cluster name).
2. **Deploy the CloudFormation stack** in AWS using those values
   (`SourceToken` / `SourceId` / `ClusterId`). It sets up Firehose log + metric
   forwarding and creates the IAM role Better Stack assumes to enumerate resources and
   manage CloudWatch log-group subscriptions. The stack outputs `IntegrationRoleArn`
   and `ExternalId`.
3. **Connect the account** by setting `aws_role_arn` + `aws_external_id` on the **same**
   `logtail_source` resource. This is a plain in-place update (`terraform apply` →
   `PATCH /api/v1/sources/:id`) that runs the exact same account-connect manager the UI
   uses — it does **not** recreate the source. Setting them later, changing them, or
   rotating credentials all behave as in-place updates (the connect/reconnect mirrors the
   UI's "paste the role ARN" step). The only `ForceNew` attribute on the resource is
   `platform`.

## Why two `terraform apply` runs (the `connect_aws_account` toggle)

Wiring step 3 directly to the stack output in a single config is a genuine dependency
cycle: the stack needs the source's `token`, and the source would need the stack's
`IntegrationRoleArn`. The example breaks it with a `connect_aws_account` variable:

```bash
terraform apply                                 # steps 1 + 2: create source + stack
terraform apply -var connect_aws_account=true   # step 3: connect the AWS account
```

This matches the two steps of the AWS CloudFormation setup in the Better Stack UI
(create the source, then paste back the role ARN).

If you prefer, you can do step 3 manually instead of via the toggle: leave the
`aws_role_arn` / `aws_external_id` lines out on the first apply, then add them (hardcoded
or from the stack outputs) and apply again — same in-place update.

## Files

- `main.tf` — the source, the CloudFormation stack, and the connect step.
- `variables.tf` — `logtail_api_token` and the `connect_aws_account` toggle.
- `outputs.tf` — source id/token and the stack's `IntegrationRoleArn`.
- `versions.tf` — provider requirements.
