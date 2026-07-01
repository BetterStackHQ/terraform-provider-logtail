# Minimal errors application config
resource "logtail_errors_application" "this" {
  name     = "Production errors"
  platform = "ruby_errors"
}

# Filed under a group, region-pinned, with retention and code mapping
# correlated to a collector's telemetry
resource "logtail_errors_application" "configured" {
  name                 = "Production errors (EU)"
  platform             = "ruby_errors"
  application_group_id = logtail_errors_application_group.this.id
  data_region          = "germany"
  errors_retention     = 90

  # Created paused, ingestion will not start unless you flip this
  ingesting_paused = true

  # Correlate errors with a source's logs and traces (immutable after creation)
  correlate_with_source_id = logtail_collector.production.source_id

  # Map container stack-trace paths to repo paths for git blame
  code_mapping_stack_root  = "/usr/src/app/"
  code_mapping_source_root = "apps/backend/"
}

# Link a connected GitHub repository for inline git blame on stack traces
resource "logtail_errors_application" "with_github" {
  name                   = "Production errors (GitHub)"
  platform               = "ruby_errors"
  github_repository_name = "BetterStackHQ/test-blame-repo"
}

# Link a connected GitLab repository instead
resource "logtail_errors_application" "with_gitlab" {
  name                   = "Production errors (GitLab)"
  platform               = "ruby_errors"
  gitlab_repository_name = "better-stack/test-blame-repo"
}
