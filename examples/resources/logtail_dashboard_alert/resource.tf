# Threshold alert notifying the team by e-mail and push
# fires when a chart value crosses a fixed limit
resource "logtail_dashboard_alert" "high_error_rate" {
  dashboard_id        = logtail_dashboard.production.id
  chart_id            = logtail_dashboard_chart.request_rate.id
  name                = "High Error Rate"
  alert_type          = "threshold"
  operator            = "higher_than"
  value               = 100
  check_period        = 60
  query_period        = 300
  confirmation_period = 60

  email = true
  push  = true
}

# Relative alert escalating to an on-call policy
# with per-series incidents, custom incident text and metadata
resource "logtail_dashboard_alert" "request_spike" {
  dashboard_id        = logtail_dashboard.production.id
  chart_id            = logtail_dashboard_chart.request_rate.id
  name                = "Request spike"
  alert_type          = "relative"
  operator            = "increases_by"
  value               = 50
  check_period        = 300
  query_period        = 3600
  recovery_period     = 300
  incident_per_series = true
  series_names        = ["500", "503"]
  incident_cause      = "{{series_name}} up {{value}} ({{operator}} {{threshold}})"

  # Pin the alert to a specific source by table name
  source_variable = "source:${logtail_source.this.table_name}"

  # Single value -> plain string; multiple values -> jsonencode([...])
  metadata = {
    runbook   = "https://example.com/runbooks/5xx"
    resolvers = jsonencode(["platform-oncall", "sre"])
  }

  escalation_target {
    # One of policy_name / policy_id / team_name / team_id
    policy_name = "My Existing Escalation Policy"
  }
}

# Anomaly alert with no fixed threshold, flagging unusual patterns
# marked critical to bypass quiet hours and notify on every channel
resource "logtail_dashboard_alert" "volume_anomaly" {
  dashboard_id         = logtail_dashboard.production.id
  chart_id             = logtail_dashboard_chart.request_rate.id
  name                 = "Request volume anomaly"
  alert_type           = "anomaly_rrcf"
  anomaly_sensitivity  = 60
  anomaly_trigger      = "higher"
  query_period         = 300
  aggregation_interval = 60

  email          = true
  push           = true
  call           = true
  sms            = true
  critical_alert = true
}

# String match on a status field - fires when the chart's latest value equals a string.
# string_value pairs with the equal/not_equal operators (no numeric value)
resource "logtail_dashboard_alert" "service_down" {
  dashboard_id = logtail_dashboard.production.id
  chart_id     = logtail_dashboard_chart.service_status.id
  name         = "Service down"
  alert_type   = "threshold"
  operator     = "equal"
  string_value = "down"
  check_period = 60

  # Created paused, alerting will not start unless you flip this
  paused = true

  email = true
  push  = true
}
