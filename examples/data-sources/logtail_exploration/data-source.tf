data "logtail_exploration" "this" {
  name = "Requests by status"

  depends_on = [logtail_exploration.this]
}
