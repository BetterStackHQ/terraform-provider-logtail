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

output "warehouse_time_series_user_events_id" {
  description = "ID of the user events time series"
  value       = logtail_warehouse_time_series.user_events.id
}

output "warehouse_time_series_response_time_id" {
  description = "ID of the response time time series"
  value       = logtail_warehouse_time_series.response_time.id
}
