data "logtail_source_group" "existing" {
  name = "My Existing Source Group"
}

output "existing_source_group_id" {
  value = data.logtail_source_group.existing.id
}
