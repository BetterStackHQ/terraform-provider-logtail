# Database target - PostgreSQL with SSL.
resource "logtail_collector_target" "primary_db" {
  collector_id = logtail_collector.production.id
  kind         = "postgres"
  host         = "10.0.0.5"
  port         = 5432
  username     = "monitor"
  password     = "example-rotate-me"
  ssl_mode     = "require"
}

# Database target - PgBouncer pooler in front of PostgreSQL.
resource "logtail_collector_target" "pooler" {
  collector_id = logtail_collector.production.id
  kind         = "pgbouncer"
  host         = "10.0.0.5"
  port         = 6432
  username     = "monitor"
  password     = "example-rotate-me"
  ssl_mode     = "disable"
}

# Database target - Elasticsearch with API key authentication.
resource "logtail_collector_target" "search" {
  collector_id = logtail_collector.production.id
  kind         = "elasticsearch"
  host         = "10.0.0.6"
  port         = 9200
  scheme       = "https"
  api_key      = "example-rotate-me"
}

# Process target - Nginx exporter on a known collector host.
resource "logtail_collector_target" "edge_nginx" {
  collector_id   = logtail_collector.production.id
  kind           = "nginx"
  service        = "edge-nginx"
  collector_host = "edge-1.internal"
  listen_ip      = "127.0.0.1"
  port           = 80
}

# Process target - custom Prometheus exporter at a full scrape URL.
resource "logtail_collector_target" "app_metrics" {
  collector_id   = logtail_collector.production.id
  kind           = "prometheus"
  service        = "my-app"
  collector_host = "app-1.internal"
  endpoint       = "http://10.0.0.5:9090/metrics"
}

# Temporarily disable a target without removing it.
resource "logtail_collector_target" "paused_replica" {
  collector_id = logtail_collector.production.id
  kind         = "postgres"
  host         = "replica.example.com"
  port         = 5432
  username     = "monitor"
  password     = "example-rotate-me"
  ssl_mode     = "disable"
  enabled      = false
}
