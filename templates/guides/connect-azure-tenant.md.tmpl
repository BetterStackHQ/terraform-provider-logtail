---
page_title: "Connecting an Azure tenant to a source"
subcategory: ""
description: |-
  Connect an Azure tenant once via the Microsoft admin consent, then link it to any number of azure sources with Terraform.
---

# Connecting an Azure tenant to a source

An `azure` platform `logtail_source` ingests logs and metrics as soon as data is forwarded to its token. Discovering resources, collecting Azure Monitor metrics, and managing diagnostic settings additionally needs a linked Azure tenant. Until a tenant is linked, Better Stack shows the "Connect your Azure tenant" step, even while ingestion works.

Unlike AWS (CloudFormation outputs) and GCP (Terraform module outputs), Azure has no credential paste-back: connecting a tenant requires the interactive **Microsoft admin consent** flow, started from any `azure` source's Ingest tab in the Better Stack UI. This is a one-time step per tenant - once consented, the tenant is available to your whole organization.

After that, link the tenant to sources with a `logtail_source_azure_account` resource:

```terraform
resource "logtail_source" "azure" {
  name     = "Azure production"
  platform = "azure"
}

resource "logtail_source_azure_account" "azure" {
  source_id        = logtail_source.azure.id
  azure_account_id = "42"
}
```

The account ID is the value of the corresponding option in the tenant picker on an `azure` source's Ingest tab (visible in the page source, or via your browser's developer tools). The same ID can be linked to any number of sources, so a single consented tenant covers your whole Terraform-managed setup.
