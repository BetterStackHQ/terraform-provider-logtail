output "logtail_source_token" {
  value = logtail_source.this.token
}

output "default_metric_expression" {
  value = data.logtail_metric.level.sql_expression
}
