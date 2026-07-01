# Dashboard templates are provided by Better Stack; look one up by name
data "logtail_dashboard_template" "hosts" {
  name = "Hosts"
}

# The template data can be passed to a logtail_dashboard to clone it
output "existing_template_size" {
  value = length(data.logtail_dashboard_template.hosts.data)
}
