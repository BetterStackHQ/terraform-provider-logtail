resource "logtail_errors_application_group" "this" {
  name = "Production errors group"
}

# Use sort_index to manually order the groups in the UI
resource "logtail_errors_application_group" "secondary" {
  name       = "My second errors group"
  sort_index = 2
}
