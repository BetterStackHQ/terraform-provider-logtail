data "logtail_errors_application" "this" {
  name = "Production errors"

  depends_on = [logtail_errors_application.this]
}
