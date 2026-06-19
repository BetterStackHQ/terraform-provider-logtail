data "logtail_source" "this" {
  table_name = logtail_source.this.table_name
}

output "existing_source_ingesting_host" {
  value = data.logtail_source.this.ingesting_host
}
