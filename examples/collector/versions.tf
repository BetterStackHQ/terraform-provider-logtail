terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "BetterStackHQ/logtail"
      version = ">= 10.9.3"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0.0"
    }
  }
}
