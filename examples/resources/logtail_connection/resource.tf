# Managing connections requires a global API token (not a team token),
# so this example is documented but excluded from the combined E2E run.
resource "logtail_connection" "example" {
  client_type = "clickhouse"
  team_names  = ["My Team"]
  data_region = "us_east"
  note        = "Example connection from Terraform"
}
