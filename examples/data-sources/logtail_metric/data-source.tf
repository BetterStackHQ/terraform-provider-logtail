# Look up a metric on a source - e.g. the built-in "level" metric.
data "logtail_metric" "level" {
  source_id = logtail_source.this.id
  name      = "level"
}

output "existing_metric_sql_expression" {
  value = data.logtail_metric.level.sql_expression
}
