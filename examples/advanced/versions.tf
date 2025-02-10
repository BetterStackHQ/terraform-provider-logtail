terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "BetterStackHQ/logtail"
      version = ">= 0.3.7"
    }
  }
}
