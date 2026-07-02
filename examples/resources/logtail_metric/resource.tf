# A metric extracted from a source's logs, queryable as a column
resource "logtail_metric" "this" {
  source_id      = logtail_source.this.id
  name           = "duration_ms"
  sql_expression = "JSONExtract(raw, 'duration_ms', 'Nullable(Float64)')"
  aggregations   = ["avg", "max", "min"]
}

# Add "histogram" to compute percentiles (p50/p90/p95/...) over the values
resource "logtail_metric" "duration_histogram" {
  source_id      = logtail_source.this.id
  name           = "duration_ms_with_histogram"
  sql_expression = "JSONExtract(raw, 'duration_ms', 'Nullable(Float64)')"
  aggregations   = ["avg", "max", "min", "histogram"]
}

# Omit aggregations to create a Label (a group-by dimension) instead of a metric
resource "logtail_metric" "service_name" {
  source_id      = logtail_source.this.id
  name           = "service_name"
  sql_expression = "JSONExtract(raw, 'service_name', 'Nullable(String)')"
}
