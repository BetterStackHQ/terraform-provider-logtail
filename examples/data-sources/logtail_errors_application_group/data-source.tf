data "logtail_errors_application_group" "existing" {
  name = "My Existing Errors Application Group"
}

output "existing_errors_application_group_id" {
  value = data.logtail_errors_application_group.existing.id
}
