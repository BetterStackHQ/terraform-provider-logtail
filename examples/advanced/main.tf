provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source_group" "group" {
  name = "Terraform Advanced Source Group"
}

resource "logtail_source" "this" {
  name               = "Terraform Advanced Source"
  platform           = "http"
  ingesting_paused   = true
  data_region        = "germany"
  live_tail_pattern  = "{level} {message}"
  logs_retention     = 60
  metrics_retention  = 90
  vrl_transformation = <<EOT
# Expected msg format: [svc:router] GET /api/health succeeded in 12.345ms
parsed, err = parse_regex(.message, r'\[svc:(?P<service>[a-zA-Z_-]+)\] .* in (?P<duration>\d+(?:\.\d+)?)ms')
if (err == null) {
    .service_name = parsed.service
    .duration_ms = to_float!(parsed.duration)
}
EOT
  source_group_id    = logtail_source_group.group.id
}

resource "logtail_metric" "duration_ms" {
  source_id      = logtail_source.this.id
  name           = "duration_ms"
  sql_expression = "getJSON(raw, 'duration_ms')"
  aggregations   = ["avg", "max", "min"]
  type           = "float64_delta"
}

resource "logtail_metric" "service_name" {
  source_id      = logtail_source.this.id
  name           = "service_name"
  sql_expression = "getJSON(raw, 'service_name')"
  aggregations   = []
  type           = "string_low_cardinality"
}

data "logtail_metric" "level" {
  source_id = logtail_source.this.id
  name      = "level"
}

resource "logtail_errors_application_group" "errors_group" {
  name = "Terraform Advanced Errors Application Group"
}

resource "logtail_errors_application" "this" {
  name                 = "Terraform Advanced Errors Application"
  platform             = "ruby_errors"
  ingesting_paused     = true
  data_region          = "germany"
  errors_retention     = 60
  application_group_id = logtail_errors_application_group.errors_group.id
}

data "logtail_errors_application" "lookup" {
  name = logtail_errors_application.this.name
}
