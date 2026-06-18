data "logtail_exploration_group" "this" {
  name = "Production explorations"

  depends_on = [logtail_exploration_group.this]
}
