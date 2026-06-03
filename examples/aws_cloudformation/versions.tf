terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "BetterStackHQ/logtail"
      version = ">= 10.14.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}
