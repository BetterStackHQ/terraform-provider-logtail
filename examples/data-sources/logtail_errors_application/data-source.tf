data "logtail_errors_application" "existing" {
  name = "My Existing Errors Application"
}

output "existing_errors_application_retention" {
  value = data.logtail_errors_application.existing.errors_retention
}

output "existing_errors_application_js_tag_token" {
  value = data.logtail_errors_application.existing.js_tag_token
}
