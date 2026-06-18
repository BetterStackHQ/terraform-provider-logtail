# A metric extracted from a source's logs, queryable as a column.
resource "logtail_metric" "this" {
  source_id      = logtail_source.this.id
  name           = "duration_ms"
  sql_expression = "getJSON(raw, 'duration_ms')"
  type           = "float64_delta"
  aggregations   = ["avg", "max", "min"]
}
