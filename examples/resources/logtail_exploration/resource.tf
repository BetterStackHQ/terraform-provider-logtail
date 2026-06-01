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

  # Select the source the queries run against. A source variable needs both
  # `values` (the sources offered) and `default_values` (the source selected by
  # default) - with values but no default_values the source is offered but not
  # selected, so the chart has no data.
  variable {
    name           = "source"
    variable_type  = "source"
    values         = [logtail_source.this.id]
    default_values = [logtail_source.this.id]
  }
}
