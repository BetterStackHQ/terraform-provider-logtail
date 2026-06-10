resource "logtail_source" "this" {
  name     = "Production logs"
  platform = "http"
}

resource "logtail_exploration" "this" {
  name            = "Requests by status"
  date_range_from = "now-24h"
  date_range_to   = "now"

  chart {
    chart_type = "bar_chart"
  }

  query {
    query_type      = "sql_expression"
    sql_query       = "SELECT {{time}} AS time, status AS series, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time, status"
    source_variable = "source"
  }

  # Select the source the queries run against.
  # `values` holds the source ID(s) the variable resolves to what the queries and any alerts run against.
  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }
}
