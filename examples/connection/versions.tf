terraform {
  required_version = ">= 0.13"
  required_providers {
    logtail = {
      source  = "BetterStackHQ/logtail"
      version = ">= 10.9.3"
    }
  }
}
