# Managing connections requires a global API token (not a team token)
data "logtail_connection" "example" {
  id = logtail_connection.example.id
}
