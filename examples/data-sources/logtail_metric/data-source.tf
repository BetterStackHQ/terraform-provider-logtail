# Look up a metric on a source — e.g. the built-in "level" metric.
data "logtail_metric" "level" {
  source_id = logtail_source.this.id
  name      = "level"
}
