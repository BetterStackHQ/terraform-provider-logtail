data "logtail_collector" "existing" {
  name = "My Existing Collector"
}

output "existing_collector_hosts_up" {
  value = data.logtail_collector.existing.hosts_up_count
}
