provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_warehouse_source_group" "group" {
  name = "Terraform Warehouse Source Group"
}

resource "logtail_warehouse_source" "this" {
  name                      = "Terraform Warehouse Source"
  data_region               = "us_east"
  events_retention          = 30
  time_series_retention     = 60
  live_tail_pattern         = "{status} {message}"
  warehouse_source_group_id = logtail_warehouse_source_group.group.id
  vrl_transformation        = <<EOT
# Make .message into an object
.message.text = .message
EOT
}

resource "logtail_warehouse_time_series" "user" {
  source_id      = logtail_warehouse_source.this.id
  name           = "user"
  type           = "string_low_cardinality"
  sql_expression = "JSONExtract(raw, 'context', 'user', 'Nullable(String)')"
  aggregations   = []
}

resource "logtail_warehouse_time_series" "message_length" {
  source_id      = logtail_warehouse_source.this.id
  name           = "message_length"
  type           = "int64_delta"
  sql_expression = "LENGTH(JSONExtractString(raw, 'message', 'text'))"
  aggregations   = ["avg", "min", "max"]
}

resource "logtail_warehouse_embedding" "message" {
  source_id  = logtail_warehouse_source.this.id
  model      = "embeddinggemma:300m"
  embed_from = "message.text"
  embed_to   = "message.embedding"
  dimension  = 512
}

resource "logtail_warehouse_time_series" "message_embedding" {
  source_id                = logtail_warehouse_source.this.id
  name                     = "message_embedding"
  type                     = "array_float32"
  sql_expression           = "JSONExtract(raw, 'message', 'embedding', 'Array(Float32)')"
  aggregations             = []
  expression_index         = "vector_similarity"
  vector_dimension         = 512
  vector_distance_function = "cosine"
}
