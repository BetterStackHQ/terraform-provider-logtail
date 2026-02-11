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
