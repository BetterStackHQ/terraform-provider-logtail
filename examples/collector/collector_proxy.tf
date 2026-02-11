# Proxy collector with HTTP Basic Auth, SSL/TLS, and custom Vector config
resource "logtail_collector" "proxy_with_auth" {
  name     = "Authenticated Proxy"
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

  user_vector_config = <<-EOT
    [sources.custom_input]
    type = "file"
    include = ["/var/log/app/*.log"]
  EOT

  configuration {
    logs_sample_rate   = 100
    traces_sample_rate = 50

    # Buffering: apply backpressure instead of dropping data
    disk_batch_size_mb   = 512
    memory_batch_size_mb = 20
    when_full            = "block"

    # VRL transformation to normalize log levels
    vrl_transformation = <<-EOT
      .level = downcase!(.level)
      .processed_at = now()
    EOT
  }
}
