output "docker_collector_id" {
  description = "ID of the Docker collector"
  value       = logtail_collector.docker_basic.id
}

output "docker_collector_secret" {
  description = "Secret token for the Docker collector"
  value       = logtail_collector.docker_basic.secret
  sensitive   = true
}

output "kubernetes_collector_status" {
  description = "Status of the Kubernetes collector"
  value       = logtail_collector.kubernetes_full.status
}

output "kubernetes_collector_source_id" {
  description = "Source ID of the Kubernetes collector"
  value       = logtail_collector.kubernetes_full.source_id
}

output "existing_collector_id" {
  description = "ID of the collector looked up via data source"
  value       = data.logtail_collector.existing.id
}

output "transformation_collector_source_id" {
  description = "Source ID for the transformation collector (use with logtail_metric)"
  value       = logtail_collector.with_transformation.source_id
}
