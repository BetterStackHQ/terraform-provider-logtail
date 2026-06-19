data "logtail_exploration" "existing" {
  name = "My Existing Exploration"
}

output "existing_exploration_id" {
  value = data.logtail_exploration.existing.id
}
