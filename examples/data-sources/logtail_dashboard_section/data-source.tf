data "logtail_dashboard_section" "performance" {
  dashboard_id = logtail_dashboard.production.id
  name         = "Performance"

  depends_on = [logtail_dashboard_section.performance]
}

output "existing_section_collapsed" {
  value = data.logtail_dashboard_section.performance.collapsed
}
