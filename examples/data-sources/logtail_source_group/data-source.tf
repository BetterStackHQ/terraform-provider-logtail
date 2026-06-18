data "logtail_source_group" "this" {
  name = "Production sources"

  depends_on = [logtail_source_group.this]
}
