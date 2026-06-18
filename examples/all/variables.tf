# Variables referenced by individual examples assembled into this directory.
# The hosts they target are not real, so the values are never used at runtime.
variable "pg_monitor_password" {
  type    = string
  default = "e2e_test_password"
}

variable "pgbouncer_monitor_password" {
  type    = string
  default = "e2e_test_password"
}

variable "es_api_key" {
  type    = string
  default = "e2e_test_password"
}
