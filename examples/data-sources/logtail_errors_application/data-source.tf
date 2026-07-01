data "logtail_errors_application" "existing" {
  name = "My Existing Errors Application"
}

output "existing_errors_application_retention" {
  value = data.logtail_errors_application.existing.errors_retention
}
