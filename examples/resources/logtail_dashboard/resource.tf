resource "logtail_dashboard" "production" {
  name               = "Production overview"
  dashboard_group_id = logtail_dashboard_group.production.id

  # Charts and alerts reference this dashboard's `source` variable via {{source}}.
  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }
}
