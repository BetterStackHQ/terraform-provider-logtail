resource "logtail_errors_application" "this" {
  name                 = "Production errors"
  platform             = "ruby_errors"
  application_group_id = logtail_errors_application_group.this.id

  # Correlate errors with a collector's logs and traces. This cannot be changed
  # after the application is created.
  correlate_with_source_id = logtail_collector.production.source_id
}
