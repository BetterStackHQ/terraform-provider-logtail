terraform {
  required_version = ">= 1.0"
  required_providers {
    logtail = {
      source  = "betterstackhq/logtail"
      version = "0.7.1"
    }
  }
}
