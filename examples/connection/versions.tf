terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "betterstackhq/logtail"
      version = "0.7.1"
    }
  }
}
