# Collector with dual-layer VRL: on-host PII redaction + server-side enrichment
resource "logtail_collector" "production" {
  name     = "Production Docker"
  platform = "docker"
  note     = "Docker hosts with PII redaction for GDPR compliance"

  logs_retention    = 30
  metrics_retention = 90

  # On-host VRL runs inside your infrastructure — raw data never leaves your network
  configuration {
    vrl_transformation = <<-EOT
      # Redact e-mail addresses
      if is_string(.message) {
        .message = replace!(.message, r'[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}', "[REDACTED_EMAIL]")
      }
      # Redact known PII fields
      if exists(.user_email) { .user_email = "[REDACTED]" }
      if exists(.client_ip) { .client_ip = "[REDACTED]" }
    EOT

    components {
      logs_host    = true
      logs_docker  = true
      ebpf_metrics = true
    }

    # Merge multi-line logs (e.g. stack traces); a new entry starts when a line matches the VRL condition
    merge_logs        = true
    merge_logs_config = "match(string!(.message), r'^\\d{4}-\\d{2}-\\d{2}')"

    # Overflow to disk after 50k in-memory events; block producers when the disk buffer is full
    buffer_max_events = 50000
    when_full         = "block"
  }

  # Server-side VRL runs during ingestion on Better Stack
  source_vrl_transformation = <<-EOT
    .environment = "production"
    .compliance_redacted = true
  EOT
}
