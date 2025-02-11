output "logtail_source_token" {
  value = logtail_source.this.token
}
output "logtail_ingesting_host" {
  value = logtail_source.this.ingesting_host
}

output "default_metric_expression" {
  value = data.logtail_metric.level.sql_expression
}
