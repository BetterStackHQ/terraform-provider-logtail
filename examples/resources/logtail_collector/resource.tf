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
  }

  # Server-side VRL runs during ingestion on Better Stack
  source_vrl_transformation = <<-EOT
    .environment = "production"
    .compliance_redacted = true
  EOT
}
