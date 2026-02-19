provider "logtail" {
  api_token = var.logtail_api_token
}

resource "random_pet" "unique" {}

# ---------------------------------------------------------------------------
# Basic Docker collector
# ---------------------------------------------------------------------------
resource "logtail_collector" "docker_basic" {
  name     = "E2E Docker ${random_pet.unique.id}"
  platform = "docker"
  note     = "Docker hosts in production environment"

  logs_retention    = 30
  metrics_retention = 90
}

# ---------------------------------------------------------------------------
# Kubernetes collector with full configuration
# ---------------------------------------------------------------------------
resource "logtail_collector" "kubernetes_full" {
  name     = "E2E Kubernetes ${random_pet.unique.id}"
  platform = "kubernetes"
  note     = "Kubernetes cluster monitoring"

  logs_retention    = 30
  metrics_retention = 60

  configuration {
    logs_sample_rate   = 100
    traces_sample_rate = 50

    components {
      ebpf_metrics      = true
      metrics_databases = true
      logs_host         = true
      logs_kubernetes   = true
      metrics_nginx     = true
    }

    namespace_option {
      name         = "production"
      log_sampling = 100
    }

    namespace_option {
      name          = "staging"
      log_sampling  = 50
      ingest_traces = true
    }

    namespace_option {
      name         = "ci"
      log_sampling = 5
    }

    # Per-service trace control
    service_option {
      name          = "payment-api"
      log_sampling  = 100
      ingest_traces = true
    }
  }
}

# ---------------------------------------------------------------------------
# Docker Swarm collector
# ---------------------------------------------------------------------------
resource "logtail_collector" "swarm" {
  name     = "E2E Swarm ${random_pet.unique.id}"
  platform = "swarm"
  note     = "Docker Swarm cluster"

  configuration {
    logs_sample_rate = 100

    components {
      logs_docker = true
      logs_host   = true
    }
  }
}

# ---------------------------------------------------------------------------
# Collector with VRL transformation (on-host + server-side)
# Demonstrates PII redaction for compliance (GDPR/HIPAA).
# ---------------------------------------------------------------------------
resource "logtail_collector" "with_transformation" {
  name     = "E2E Transformation ${random_pet.unique.id}"
  platform = "docker"
  note     = "PII redaction for GDPR/HIPAA compliance"

  # On-host VRL: PII never leaves your network
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
    EOT
  }

  # Server-side VRL: enrich during ingestion on Better Stack
  source_vrl_transformation = <<-EOT
    .environment = "production"
    .compliance_redacted = true
  EOT
}

# ---------------------------------------------------------------------------
# Collector with database monitoring
# ---------------------------------------------------------------------------
resource "logtail_collector" "with_databases" {
  name     = "E2E Databases ${random_pet.unique.id}"
  platform = "docker"
  note     = "Collector with database monitoring"

  databases {
    service_type = "postgres"
    host         = "db.example.com"
    port         = 5432
    username     = "collector"
    password     = var.db_password
    ssl_mode     = "require"
  }

  databases {
    service_type = "redis"
    host         = "redis.example.com"
    port         = 6379
  }
}

# ---------------------------------------------------------------------------
# Proxy collector with auth, SSL, and custom Vector YAML
# ---------------------------------------------------------------------------
resource "logtail_collector" "proxy_with_auth" {
  name     = "E2E Proxy ${random_pet.unique.id}"
  platform = "proxy"
  note     = "Proxy collector with authentication and custom SSL"

  proxy_config {
    enable_http_basic_auth    = true
    http_basic_auth_username  = "api_user"
    http_basic_auth_password  = var.proxy_password
    enable_ssl_certificate    = true
    ssl_certificate_host      = "logs.example.com"
    enable_buffering_proxy    = true
    buffering_proxy_listen_on = "0.0.0.0:8080"
  }

  # Looking for sinks to send data to Better Stack?
  # The sink `better_stack_http_logs_sink` in generated.vector.yaml will automatically pick up any source or transform with a name starting with `better_stack_logs_`.
  # Sinks `better_stack_http_traces_sink` and `better_stack_http_metrics_sink` will pick up any source or transform with a name starting with `better_stack_traces_` and `better_stack_metrics_` respectively.
  user_vector_config = <<-EOT
    sources:
      better_stack_logs_my_custom_input:
        type: file
        include:
          - "/var/log/app/*.log"

      better_stack_traces_sample_input:
        type: file
        include:
          - /dev/null

      better_stack_metrics_sample_input:
        type: static_metrics
  EOT

  configuration {
    logs_sample_rate   = 100
    traces_sample_rate = 50

    disk_batch_size_mb   = 512
    memory_batch_size_mb = 20

    # VRL transformation to normalize log levels
    vrl_transformation = <<-EOT
      .level = downcase!(.level)
      .processed_at = now()
    EOT
  }
}

# ---------------------------------------------------------------------------
# Data source to look up a collector created above
# ---------------------------------------------------------------------------
data "logtail_collector" "existing" {
  name       = logtail_collector.docker_basic.name
  depends_on = [logtail_collector.docker_basic]
}

# ---------------------------------------------------------------------------
# Use source_id with logtail_metric to define metrics on collector data
# ---------------------------------------------------------------------------
resource "logtail_metric" "docker_error_rate" {
  source_id      = logtail_collector.docker_basic.source_id
  name           = "duration_ms"
  sql_expression = "getJSON(raw, 'duration_ms')"
  type           = "float64_delta"
  aggregations   = ["avg", "max", "min"]
}
