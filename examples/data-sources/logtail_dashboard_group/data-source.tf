data "logtail_dashboard_group" "existing" {
  name = "My Existing Dashboard Group"
}

output "existing_dashboard_group_id" {
  value = data.logtail_dashboard_group.existing.id
}
