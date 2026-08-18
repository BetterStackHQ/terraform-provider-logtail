# Register a release the moment it deploys, before its first error arrives
resource "logtail_errors_release" "this" {
  application_id = logtail_errors_application.this.id
  version        = "v1.0.0"
  environments   = ["production"]
}
