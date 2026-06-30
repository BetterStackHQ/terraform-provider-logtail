# Collector with dual-layer VRL (on-host PII redaction + server-side enrichment)
# and database monitoring targets.
resource "logtail_collector" "production" {
  name     = "Production Docker"
  platform = "docker"
  note     = "Docker hosts with PII redaction for GDPR compliance"

  logs_retention    = 30
  metrics_retention = 90

  # On-host VRL runs inside your infrastructure - raw data never leaves your network
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

# Kubernetes collector with per-namespace and per-service sampling.
resource "logtail_collector" "kubernetes" {
  name     = "Production Kubernetes"
  platform = "kubernetes"

  logs_retention    = 30
  metrics_retention = 60

  configuration {
    logs_sample_rate         = 100
    traces_sample_rate       = 50
    log_line_length_limit_kb = 32

    buffer_max_events = 50000
    when_full         = "block"

    components {
      ebpf_metrics      = true
      metrics_databases = true
      logs_host         = true
      logs_kubernetes   = true
      metrics_nginx     = true
    }

    # Per-namespace log sampling and trace ingestion.
    namespace_option {
      name         = "production"
      log_sampling = 100
    }

    namespace_option {
      name          = "staging"
      log_sampling  = 50
      ingest_traces = true
    }

    # Per-service trace control.
    service_option {
      name          = "payment-api"
      log_sampling  = 100
      ingest_traces = true
    }
  }
}

# Docker Swarm collector.
resource "logtail_collector" "swarm" {
  name     = "Production Swarm"
  platform = "swarm"

  configuration {
    logs_sample_rate = 100

    components {
      logs_docker = true
      logs_host   = true
    }
  }
}
