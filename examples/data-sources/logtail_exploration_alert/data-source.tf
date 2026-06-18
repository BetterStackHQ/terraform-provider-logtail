data "logtail_exploration_alert" "errors_high" {
  exploration_id = logtail_exploration.this.id
  name           = "Too many errors"

  depends_on = [logtail_exploration_alert.errors_high]
}
