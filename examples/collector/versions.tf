terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "BetterStackHQ/logtail"
      version = ">= 0.8.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0.0"
    }
  }
}
