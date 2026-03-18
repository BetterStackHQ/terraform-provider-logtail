# A line chart showing request rate over time
resource "logtail_dashboard_chart" "request_rate" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "line_chart"
  name         = "Request Rate"
  description  = "Requests per second by endpoint"
  x            = 0
  y            = 0
  w            = 6
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
  }
}

# A number chart showing total error count
resource "logtail_dashboard_chart" "error_count" {
  dashboard_id = logtail_dashboard.production.id
  chart_type   = "number_chart"
  name         = "Total Errors"
  x            = 6
  y            = 0
  w            = 3
  h            = 4

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} AND level = 'error'"
  }
}
