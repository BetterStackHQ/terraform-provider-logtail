# Minimal Docker collector config
resource "logtail_collector" "production" {
  name     = "Production Docker"
  platform = "docker"
}

# Dual-layer VRL with on-host PII redaction and server-side enrichment
# plus log merging, disk buffering, retention, region and folder placement
resource "logtail_collector" "compliance" {
  name              = "Compliance Docker"
  platform          = "docker"
  note              = "Docker hosts with PII redaction for GDPR compliance"
  data_region       = "germany"
  logs_retention    = 30
  metrics_retention = 90
  source_group_id   = logtail_source_group.this.id
  live_tail_pattern = "{level} {message}"

  # On-host VRL runs inside your infrastructure - raw data never leaves your network
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
      # Redact known PII fields
      if exists(.user_email) { .user_email = "[REDACTED]" }
      if exists(.client_ip) { .client_ip = "[REDACTED]" }
    EOT

    components {
      logs_host          = true
      logs_docker        = true
      ebpf_metrics       = true
      ebpf_tracing_basic = true
    }

    # Merge multi-line logs such as stack traces
    # a new entry starts when a line matches the condition
    merge_logs        = true
    merge_logs_config = "match(string!(.message), r'^\\d{4}-\\d{2}-\\d{2}')"

    # Overflow to disk after 50k in-memory events
    # block producers when the disk buffer is full
    buffer_max_events = 50000
    when_full         = "block"

    # Outgoing request batches: disk buffer >= 256 MB, memory batch <= 40 MB
    disk_batch_size_mb   = 256
    memory_batch_size_mb = 10
  }

  # Server-side VRL runs during ingestion on Better Stack
  source_vrl_transformation = <<-EOT
    .environment = "production"
    .compliance_redacted = true
  EOT
}

# Kubernetes collector with per-namespace and per-service sampling
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
      ebpf_metrics             = true
      ebpf_red_metrics         = true
      ebpf_tracing_full        = true
      metrics_databases        = true
      metrics_nginx            = true
      metrics_apache           = true
      logs_host                = false
      logs_kubernetes          = true
      logs_collector_internals = false
      traces_opentelemetry     = true
    }

    # Per-namespace log sampling and trace ingestion
    namespace_option {
      name         = "production"
      log_sampling = 100
    }

    namespace_option {
      name          = "staging"
      log_sampling  = 50
      ingest_traces = true
    }

    # Per-service trace control
    service_option {
      name          = "payment-api"
      log_sampling  = 100
      ingest_traces = true
    }
  }
}

# Docker Swarm collector
resource "logtail_collector" "swarm" {
  name     = "Production Swarm"
  platform = "swarm"

  # Created paused, collecting data will not start unless you flip this
  ingesting_paused = true

  configuration {
    logs_sample_rate = 100

    components {
      logs_docker = true
      logs_host   = true
    }
  }
}

# Extra Docker collector forwarding a custom log file via a Vector source
# sources named better_stack_logs_* are picked up by the logs sink automatically
resource "logtail_collector" "custom_sources" {
  name     = "Custom sources Docker"
  platform = "docker"

  user_vector_config = <<-EOT
    sources:
      better_stack_logs_custom_file:
        type: file
        include:
          - /host/var/log/custom.log
  EOT
}
