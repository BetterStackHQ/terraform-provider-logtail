terraform {
  required_providers {
    logtail = {
      source = "BetterStackHQ/logtail"
    }
  }
}

variable "logtail_api_token" {
  description = "Better Stack Telemetry API Token"
  type        = string
  sensitive   = true
}

variable "db_password" {
  description = "Database password for collector monitoring"
  type        = string
  sensitive   = true
  default     = ""
}

variable "proxy_password" {
  description = "Password for HTTP Basic Auth on proxy collector"
  type        = string
  sensitive   = true
  default     = ""
}

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

  logs_retention    = 14
  metrics_retention = 60

  configuration {
    logs_sample_rate   = 100
    traces_sample_rate = 50

    collector_components {
      beyla         = true
      cluster_agent = true
      host_logs     = true
    }

    monitoring_options {
      collector_kubernetes = true
      nginx_metrics        = true
    }
  }
}

# Collector with database monitoring
resource "logtail_collector" "with_databases" {
  name     = "App Server Collector"
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

# Collector with custom S3 bucket storage
resource "logtail_collector" "with_custom_bucket" {
  name     = "Archive Collector"
  platform = "docker"
  note     = "Collector with custom S3 storage"

  logs_retention    = 365
  metrics_retention = 365

  custom_bucket {
    name                      = "my-logs-archive-bucket"
    endpoint                  = "https://s3.us-east-1.amazonaws.com"
    access_key_id             = "AKIAIOSFODNN7EXAMPLE"
    secret_access_key         = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    keep_data_after_retention = true
  }
}

# Proxy collector with HTTP Basic Auth and SSL/TLS
resource "logtail_collector" "proxy_with_auth" {
  name     = "Authenticated Proxy"
  platform = "proxy"
  note     = "Proxy collector with authentication and custom SSL"

  # HTTP Basic Auth for securing the proxy endpoint
  enable_http_basic_auth   = true
  http_basic_auth_username = "api_user"
  http_basic_auth_password = var.proxy_password

  configuration {
    logs_sample_rate   = 100
    traces_sample_rate = 50

    # VRL transformation to normalize log levels
    transformation = <<-EOT
      .level = downcase!(.level)
      .processed_at = now()
    EOT

    # Custom SSL/TLS certificate settings
    enable_ssl_certificate = true
    ssl_certificate_host   = "logs.example.com"
  }
}

# Collector with VRL transformation only
resource "logtail_collector" "with_transformation" {
  name     = "Transformation Collector"
  platform = "docker"
  note     = "Collector with VRL transformation"

  configuration {
    transformation = <<-EOT
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

# Data source to look up an existing collector
data "logtail_collector" "existing" {
  name = "My Existing Collector"
}

# Outputs
output "docker_collector_id" {
  description = "ID of the Docker collector"
  value       = logtail_collector.docker_basic.id
}

output "docker_collector_secret" {
  description = "Secret token for the Docker collector"
  value       = logtail_collector.docker_basic.secret
  sensitive   = true
}

output "kubernetes_collector_status" {
  description = "Status of the Kubernetes collector"
  value       = logtail_collector.kubernetes_full.status
}

output "existing_collector_hosts_count" {
  description = "Number of hosts connected to the existing collector"
  value       = data.logtail_collector.existing.hosts_count
}
