# Link a source to a GCP project you've already connected, by account ID
# gcp_account_id is write-only - the API never returns it, so it isn't refreshed
resource "logtail_source" "gcp_existing" {
  name     = "GCP staging"
  platform = "gcp"
}

resource "logtail_source_gcp_project" "existing" {
  source_id      = logtail_source.gcp_existing.id
  gcp_account_id = "42"
}

# Full setup in one terraform apply, creating the source and configuring
# Workload Identity Federation via the Better Stack GCP Terraform module
resource "logtail_source" "gcp" {
  name     = "GCP production"
  platform = "gcp"
}

module "better_stack" {
  source         = "github.com/BetterStackHQ/gcp//terraform"
  project_id     = "my-gcp-project"
  source_token   = logtail_source.gcp.token
  ingesting_host = logtail_source.gcp.ingesting_host
}

resource "logtail_source_gcp_project" "gcp" {
  source_id          = logtail_source.gcp.id
  gcp_project_id     = module.better_stack.project_id
  gcp_project_number = module.better_stack.project_number
}
