# Managing connections requires a global API token (not a team token)
resource "logtail_connection" "example" {
  client_type = "clickhouse"
  team_names  = ["My Team"]
}

# Lock a connection down: pin the region, restrict by IP, set an expiry, add a note
resource "logtail_connection" "restricted" {
  client_type  = "clickhouse"
  team_names   = ["My Team"]
  data_region  = "germany"
  note         = "Read-only analytics connection"
  ip_allowlist = ["203.0.113.0/24", "198.51.100.10"]
  valid_until  = "2030-01-01T00:00:00Z"
}

# Associate teams by ID instead of name (provide one of team_names or team_ids)
resource "logtail_connection" "by_team_id" {
  client_type = "clickhouse"
  team_ids    = [123456]
}
