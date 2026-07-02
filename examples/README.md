# Example usage
[![build](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/build.yml)
[![tests](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/test.yml)
[![documentation](https://img.shields.io/badge/-documentation-blue)](https://registry.terraform.io/providers/BetterStackHQ/logtail/latest/docs)

These examples demonstrate how to provision and manage Better Stack Telemetry resources with Terraform.

The files in this directory are a ready-to-run **basic** example - a source, an errors application, a couple of explorations and an alert.

The [`connection/`](./connection) directory shows a connection, which requires a global API token.

Detailed, per-resource examples live in [`resources/`](./resources) and [`data-sources/`](./data-sources) - these are embedded in the registry docs and exercised end-to-end in CI.

## Usage

```shell script
git clone https://github.com/BetterStackHQ/terraform-provider-logtail && \
  cd terraform-provider-logtail/examples

echo '# See variables.tf for more.
logtail_api_token = "XXXXXXXXXXXXXXXXXXXXXXXX"
' > terraform.tfvars

terraform init
terraform apply

# The source token to start shipping logs, and the host to send them to:
terraform output logtail_source_token
terraform output logtail_ingesting_host
```

## Documentation

See [Better Stack Telemetry API docs](https://betterstack.com/docs/logs/api/getting-started/) to obtain API token and get the complete list of parameter options.
Or explore the [Terraform Registry provider documentation](https://registry.terraform.io/providers/BetterStackHQ/logtail/latest/docs).
