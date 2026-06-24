resource "logtail_source" "this" {
  name     = "Production"
  platform = "http"

  # Server-side VRL transformations are applied per telemetry type during ingestion.
  # vrl_transformation_logs runs on logs; vrl_transformation_spans runs on traces.
  vrl_transformation_logs = <<-EOT
    .level = downcase(to_string!(.level))
  EOT

  vrl_transformation_spans = <<-EOT
    # Drop noisy health-check spans
    if .name == "GET /api/health" {
        abort
    }
  EOT
}
