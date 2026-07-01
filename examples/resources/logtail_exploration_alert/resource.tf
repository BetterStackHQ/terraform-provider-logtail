# Threshold alert notifying the team by e-mail and push
# fires when the value crosses a fixed limit
resource "logtail_exploration_alert" "errors_high" {
  exploration_id      = logtail_exploration.this.id
  name                = "Too many errors"
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
resource "logtail_exploration_alert" "errors_spike" {
  exploration_id      = logtail_exploration.this.id
  name                = "5xx spike"
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
resource "logtail_exploration_alert" "volume_anomaly" {
  exploration_id       = logtail_exploration.this.id
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

# String match across every HTTP source rather than a single source variable
# string_value pairs with the equal/not_equal operators
resource "logtail_exploration_alert" "status_watch" {
  exploration_id   = logtail_exploration.this.id
  name             = "Unexpected status value"
  alert_type       = "threshold"
  operator         = "not_equal"
  string_value     = "200"
  check_period     = 300
  source_mode      = "platforms_all_sources"
  source_platforms = ["http"]

  # Created paused, alerting will not start unless you flip this
  paused = true

  email = true
}
