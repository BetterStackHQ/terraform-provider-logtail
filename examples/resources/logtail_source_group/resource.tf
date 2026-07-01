resource "logtail_source_group" "this" {
  name = "Production sources"
}

# Use sort_index to manually order the groups in the UI
resource "logtail_source_group" "secondary" {
  name       = "My second source group"
  sort_index = 2
}
