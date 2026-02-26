provider "logtail" {
  api_token = var.logtail_api_token
}

# =============================================================================
# Sources
# =============================================================================

resource "logtail_source" "this" {
  name     = "Terraform Basic Source"
  platform = "http"
}

resource "logtail_errors_application" "this" {
  name     = "Terraform Basic Errors Application"
  platform = "ruby_errors"
}

# =============================================================================
# Explorations - Basic Examples
# =============================================================================

# Line chart - event count over time
resource "logtail_exploration" "event_count" {
  name = "Event Count Over Time"

  chart {
    chart_type  = "line_chart"
    description = "Shows the number of events over time"
  }

  query {
    query_type = "sql_expression"
    sql_query  = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time"
  }
}

# Tail chart - live log view with filter
resource "logtail_exploration" "error_logs" {
  name = "Terraform Explore Errors"

  chart {
    chart_type  = "tail_chart"
    description = "Live view of error logs"
  }

  query {
    query_type      = "tail_query"
    where_condition = "level=error"
  }

  variable {
    name = "source"
    variable_type = "source"
    values = [logtail_source.this.id]
  }
}

# =============================================================================
# Simple Alert
# =============================================================================

resource "logtail_exploration_alert" "error_logs" {
  exploration_id = logtail_exploration.error_logs.id
  name           = "Terraform Errors Alert"
  alert_type     = "threshold"
  operator       = "higher_than"
  value          = 0
  check_period   = 60

  email = true
}
