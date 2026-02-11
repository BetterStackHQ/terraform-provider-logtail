# Compliance-focused collector with PII redaction
# On-host VRL strips sensitive data BEFORE it leaves your infrastructure.
# Server-side VRL enriches data during ingestion on Better Stack.
resource "logtail_collector" "compliance" {
  name     = "Compliance Collector"
  platform = "docker"
  note     = "PII redaction for GDPR/HIPAA compliance"

  # On-host: PII never leaves your network
  configuration {
    vrl_transformation = <<-EOT
      # Redact e-mail addresses
      if is_string(.message) {
        .message = replace!(.message, r'[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}', "[REDACTED_EMAIL]")
      }

      # Redact IPv4 addresses
      if is_string(.message) {
        .message = replace!(.message, r'\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b', "[REDACTED_IP]")
      }

      # Redact credit card numbers
      if is_string(.message) {
        .message = replace!(.message, r'\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b', "[REDACTED_CC]")
      }

      # Redact known PII fields
      if exists(.user_email) { .user_email = "[REDACTED]" }
      if exists(.client_ip) { .client_ip = "[REDACTED]" }
    EOT
  }

  # Server-side: enrich during ingestion
  source_vrl_transformation = <<-EOT
    .environment = "production"
    .compliance_redacted = true
  EOT
}
