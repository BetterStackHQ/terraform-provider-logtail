provider "logtail" {
  api_token = var.logtail_api_token
}

resource "logtail_source" "this" {
  name     = "Terraform Basic Source"
  platform = "http"
}

resource "logtail_errors_application" "this" {
  name     = "Terraform Basic Errors Application"
  platform = "ruby_errors"
}

resource "logtail_exploration" "this" {
  name = "Terraform Basic Exploration"

  chart {
    chart_type = "line_chart"
  }

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
  }
}
