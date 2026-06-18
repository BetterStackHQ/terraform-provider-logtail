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
    policy_name = "My Existing Escalation Policy"
  }
}
