# Managing connections requires a global API token (not a team token)
resource "logtail_connection" "example" {
  client_type = "clickhouse"
  team_names  = ["My Team"]
  data_region = "us_east"
  note        = "Example connection from Terraform"
}
