# Link a source to an Azure tenant you've already connected, by account ID.
# Unlike AWS and GCP there is no credential paste-back: connect the tenant once
# via the Microsoft admin consent in the Better Stack UI, then link it to any
# number of sources. azure_account_id is write-only - the API never returns it,
# so it isn't refreshed from state.
resource "logtail_source" "azure" {
  name     = "Azure production"
  platform = "azure"
}

resource "logtail_source_azure_account" "azure" {
  source_id        = logtail_source.azure.id
  azure_account_id = "42"
}
