resource "logtail_source" "this" {
  name     = "Production logs"
  platform = "http"
}

resource "logtail_exploration" "this" {
  name = "Errors over time"

  chart {
    chart_type = "line_chart"
  }

  query {
    query_type      = "sql_expression"
    sql_query       = "SELECT {{time}} AS time, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} AND level = 'error' GROUP BY time"
    source_variable = "source"
  }

  # A source variable needs both `values` (selectable sources) and
  # `default_values` (the selected source); otherwise the source is offered but
  # not selected and the alert has no data to evaluate.
  variable {
    name           = "source"
    variable_type  = "source"
    values         = [logtail_source.this.id]
    default_values = [logtail_source.this.id]
  }
}

# Threshold alert notifying the current team by e-mail and push.
resource "logtail_exploration_alert" "errors_high" {
  exploration_id = logtail_exploration.this.id
  name           = "Too many errors"
  alert_type     = "threshold"
  operator       = "higher_than"
  value          = 100
  check_period   = 60

  email = true
  push  = true
}

# Relative alert that escalates to a specific on-call policy. When an escalation
# policy is set, the policy controls notifications (call/sms/email/push).
resource "logtail_exploration_alert" "errors_spike" {
  exploration_id = logtail_exploration.this.id
  name           = "Error spike"
  alert_type     = "relative"
  operator       = "increases_by"
  value          = 50
  check_period   = 300

  escalation_target {
    policy_name = "Engineering on-call"
  }
}
