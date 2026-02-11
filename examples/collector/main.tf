provider "logtail" {
  api_token = var.logtail_api_token
}

# Basic Docker collector
resource "logtail_collector" "docker_basic" {
  name     = "Production Docker"
  platform = "docker"
  note     = "Docker hosts in production environment"

  logs_retention    = 30
  metrics_retention = 90
}

# Kubernetes collector with full configuration
resource "logtail_collector" "kubernetes_full" {
  name     = "Production Kubernetes"
  platform = "kubernetes"
  note     = "Kubernetes cluster monitoring"

  logs_retention    = 7
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

# Collector with VRL transformation
resource "logtail_collector" "with_transformation" {
  name     = "Transformation Collector"
  platform = "docker"
  note     = "Collector with VRL transformation"

  configuration {
    vrl_transformation = <<-EOT
      # Add metadata
      .source = "docker"
      .environment = "production"

      # Parse JSON if message is JSON
      if is_string(.message) {
        .parsed = parse_json(.message) ?? null
      }
    EOT
  }
}

# Data source to look up a collector created above
data "logtail_collector" "existing" {
  name       = "Production Docker"
  depends_on = [logtail_collector.docker_basic]
}
