provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source_group" "group" {
  name = "Terraform Advanced Source Group"
}

resource "logtail_source" "this" {
  name              = "Terraform Advanced Source"
  platform          = "http"
  ingesting_paused  = true
  data_region       = "germany"
  live_tail_pattern = "{level} {message}"
  logs_retention    = 60
  metrics_retention = 90
  source_group_id   = logtail_source_group.group.id
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
