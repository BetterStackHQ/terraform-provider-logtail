data "logtail_errors_application_group" "this" {
  name = "Production errors group"

  depends_on = [logtail_errors_application_group.this]
}
