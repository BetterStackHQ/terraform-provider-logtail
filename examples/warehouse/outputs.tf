output "warehouse_source_group_id" {
  description = "ID of the created warehouse source group"
  value       = logtail_warehouse_source_group.group.id
}

output "warehouse_source_id" {
  description = "ID of the created warehouse source"
  value       = logtail_warehouse_source.this.id
}

output "warehouse_source_token" {
  description = "Token of the created warehouse source"
  value       = logtail_warehouse_source.this.token
  sensitive   = true
}

output "warehouse_embedding_message_id" {
  description = "ID of the message embedding"
  value       = logtail_warehouse_embedding.message.id
}

output "warehouse_time_series_message_embedding_id" {
  description = "ID of the message embedding time series with vector index"
  value       = logtail_warehouse_time_series.message_embedding.id
}
