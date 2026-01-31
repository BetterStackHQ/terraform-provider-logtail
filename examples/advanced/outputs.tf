output "logtail_source_token" {
  value = logtail_source.this.token
}
output "logtail_ingesting_host" {
  value = logtail_source.this.ingesting_host
}
output "logtail_data_region" {
  value = logtail_source.this.data_region
}

output "default_metric_expression" {
  value = data.logtail_metric.level.sql_expression
}

output "dashboard_template_host_overview_id" {
  value = data.logtail_dashboard_template.host_overview.id
}

output "copied_dashboard_url" {
  value = "https://telemetry.betterstack.com/team/${logtail_dashboard.from_template.team_id}/dashboards/${logtail_dashboard.from_template.id}"
}

output "custom_dashboard_url" {
  value = "https://telemetry.betterstack.com/team/${logtail_dashboard.custom.team_id}/dashboards/${logtail_dashboard.custom.id}"
}

output "dashboard_host_prometheus_size" {
  value = length(data.logtail_dashboard.host_prometheus.data)
}
