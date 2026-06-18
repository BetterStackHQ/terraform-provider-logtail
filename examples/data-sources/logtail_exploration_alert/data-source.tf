data "logtail_exploration_alert" "errors_high" {
  exploration_id = logtail_exploration.this.id
  name           = logtail_exploration_alert.errors_high.name
}
