data "logtail_exploration_group" "existing" {
  name = "My Existing Exploration Group"
}

output "existing_exploration_group_id" {
  value = data.logtail_exploration_group.existing.id
}
