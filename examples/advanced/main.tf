provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "this" {
  name              = "Terraform Advanced Source"
  platform          = "http"
  ingesting_paused  = true
  live_tail_pattern = "{level} {message}"
  logs_retention    = 60
  metrics_retention = 90
}

resource "logtail_metric" "duration_ms" {
  source_id      = logtail_source.this.id
  name           = "duration_ms"
  sql_expression = "JSONExtract(json, 'duration_ms', 'Nullable(Float)')"
  aggregations   = ["avg", "max", "min"]
  type           = "float64_delta"
}

resource "logtail_metric" "service_name" {
  source_id      = logtail_source.this.id
  name           = "service_name"
  sql_expression = "JSONExtract(json, 'service_name', 'Nullable(String)')"
  aggregations   = []
  type           = "string_low_cardinality"
}

data "logtail_metric" "level" {
  source_id = logtail_source.this.id
  name      = "level"
}
