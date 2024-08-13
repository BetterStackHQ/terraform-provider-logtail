# terraform-provider-logtail [![build](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/BetterStackHQ/terraform-provider-logtail/actions/workflows/build.yml) [![documentation](https://img.shields.io/badge/-documentation-blue)](https://registry.terraform.io/providers/BetterStackHQ/logtail/latest/docs)

Terraform (0.13+) provider for [Better Stack Logs](https://betterstack.com/logs) (formerly *Logtail.com*).

## Installation

```terraform
terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source = "BetterStackHQ/logtail"
      version = ">= 0.2.0"
    }
  }
}
```

## Example Usage

```terraform
provider "logtail" {
  # `api_token` can be omitted if LOGTAIL_API_TOKEN env var is set.
  api_token = "XXXXXXXXXXXXXXXXXXXXXXXX"
}

resource "logtail_source" "this" {
  name     = "Production Server"
  platform = "ubuntu"
}

output "logtail_source_token" {
  value = logtail_source.this.token
}
```

## Documentation

See Terraform Registry [docs](https://registry.terraform.io/providers/BetterStackHQ/logtail/latest/docs).

## Development

> PREREQUISITE: [go1.16+](https://golang.org/dl/).

```shell script
git clone https://github.com/betterstackhq/terraform-provider-logtail && \
  cd terraform-provider-logtail

make help
```

## Releasing New Versions

Simply push a new tag `vX.Y.Z` to GitHub and a new version will be built and released automatically through a GitHub action.
