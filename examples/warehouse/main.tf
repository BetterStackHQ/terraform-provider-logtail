provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_warehouse_source_group" "group" {
  name = "Terraform Warehouse Source Group"
}

resource "logtail_warehouse_source" "this" {
  name                   = "Terraform Warehouse Source"
  data_region            = "us_east"
  events_retention       = 30
  time_series_retention  = 60
  live_tail_pattern      = "{status} {message}"
  source_group_id        = logtail_warehouse_source_group.group.id
  vrl_transformation     = <<EOT
# Transform warehouse events
.user_id = getJSON(.raw, "user_id")
.event_type = getJSON(.raw, "event_type")
EOT
}

resource "logtail_warehouse_time_series" "user_events" {
  source_id       = logtail_warehouse_source.this.id
  name            = "user_events"
  type            = "string_low_cardinality"
  sql_expression  = "JSONExtract(raw, 'event_type', 'Nullable(String)')"
  aggregations    = []
}

resource "logtail_warehouse_time_series" "response_time" {
  source_id       = logtail_warehouse_source.this.id
  name            = "response_time"
  type            = "float64_delta"
  sql_expression  = "JSONExtract(raw, 'response_time', 'Nullable(Float64)')"
  aggregations    = ["avg", "min", "max"]
}

data "logtail_warehouse_source" "lookup" {
  name = logtail_warehouse_source.this.name
}

data "logtail_warehouse_source_group" "lookup" {
  name = logtail_warehouse_source_group.group.name
}
