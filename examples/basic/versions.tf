terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source = "registry.terraform.io/betterstack/logtail"
      # https://github.com/betterstackhq/terraform-provider-logtail/blob/master/CHANGELOG.md
      version = "0.0.0-0"
    }
  }
}
