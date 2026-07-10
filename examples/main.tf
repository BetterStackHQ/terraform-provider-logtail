resource "logtail_source" "this" {
  name               = "Terraform Advanced Source"
  platform           = "http"
  ingesting_paused   = true
  data_region        = "germany"
  live_tail_pattern  = "{level} {message}"
  logs_retention     = 60
  metrics_retention  = 90
  vrl_transformation = <<EOT
# Expected msg format: [svc:router] GET /api/health succeeded in 12.345ms
parsed, err = parse_regex(.message, r'\[svc:(?P<service>[a-zA-Z_-]+)\] .* in (?P<duration>\d+(?:\.\d+)?)ms')
if (err == null) {
    .service_name = parsed.service
    .duration_ms = to_float!(parsed.duration)
}
EOT
  custom_bucket {
    name                      = "my-test-bucket2"
    endpoint                  = "https://7faa830e77ada78a80b015875a7e1e3e.r2.cloudflarestorage.com/telemetry-e2e-test-bucket"
    access_key_id             = "19701543064d2c698cbda7c6b091d735"
    secret_access_key         = "adacaec2310c7a92369e2ec28152871125264ef3cc6f5d28deba7f77f0b37462"
    keep_data_after_retention = true
  }
}
