# Managing connections requires a global API token (not a team token),
# so this example is documented but excluded from the combined E2E run.
data "logtail_connection" "example" {
  id = logtail_connection.example.id
}
