# Minimal line chart config
resource "logtail_dashboard_chart" "request_rate" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "line_chart"
  name         = "Request Rate"
  description  = "Request volume over time"
  x            = 0
  y            = 0
  w            = 6
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, countMerge(events_count) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}} GROUP BY time ORDER BY time"
  }
}

# A line chart with explicit display settings (units, legend, missing-value handling)
resource "logtail_dashboard_chart" "requests_by_status" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "line_chart"
  name         = "Requests by status"
  x            = 6
  y            = 0
  w            = 6
  h            = 4

  settings = jsonencode({
    unit                 = "shortened"
    legend               = "shown_below"
    stacking             = "dont_stack"
    decimal_places       = 0
    treat_missing_values = "connected"
  })

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, label('response_status') AS series, countMerge(events_count) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}} GROUP BY time, series ORDER BY time"
  }
}

# A number chart showing a single value
resource "logtail_dashboard_chart" "error_count" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "number_chart"
  name         = "Total Errors"
  x            = 0
  y            = 4
  w            = 3
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT countMerge(events_count) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}} AND label('level') = 'error'"
  }
}

# A static-text chart for notes and links (no query against a source)
resource "logtail_dashboard_chart" "notes" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "static_text_chart"
  name         = "Runbook"
  x            = 3
  y            = 4
  w            = 3
  h            = 4

  query {
    query_type  = "static_text"
    static_text = "## Runbook\n\nSee the on-call guide before acting on these charts."
  }
}

# Latest service status as a single value - read by the service_down alert
resource "logtail_dashboard_chart" "service_status" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "number_chart"
  name         = "Service status"
  x            = 6
  y            = 4
  w            = 3
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT anyLast(label('service_status')) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}}"
  }
}

# Chart on the tuned dashboard driven by its variables - the [[ ]] blocks apply
# a filter only when the environment / level variable has a value selected
resource "logtail_dashboard_chart" "tuned_events" {
  dashboard_id = logtail_dashboard.tuned.id
  chart_type   = "line_chart"
  name         = "Events over time"
  x            = 0
  y            = 0
  w            = 9
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, countMerge(events_count) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}} [[ AND label('environment') = {{environment}} ]] [[ AND label('level') = {{level}} ]] GROUP BY time ORDER BY time"
  }
}

# Error count for the selected environment
resource "logtail_dashboard_chart" "tuned_errors" {
  dashboard_id = logtail_dashboard.tuned.id
  chart_type   = "number_chart"
  name         = "Errors"
  x            = 9
  y            = 0
  w            = 3
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT countMerge(events_count) AS value FROM {{source}} WHERE {{time}} BETWEEN {{start_time}} AND {{end_time}} AND label('level') = 'error' [[ AND label('environment') = {{environment}} ]]"
  }
}

# Live tail chart - where_condition filters the stream
resource "logtail_dashboard_chart" "live_errors" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "tail_chart"
  name         = "Live errors"
  x            = 0
  y            = 8
  w            = 12
  h            = 6

  query {
    query_type      = "tail_query"
    where_condition = "level=error"
    source_variable = "source"
  }
}
