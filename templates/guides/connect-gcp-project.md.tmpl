---
page_title: "Connecting a GCP project to a source"
subcategory: ""
description: |-
  Create a GCP source, configure Workload Identity Federation with the Better Stack GCP Terraform module, and link the project in a single terraform apply.
---

# Connecting a GCP project to a source

A `gcp` platform `logtail_source` ingests logs and metrics as soon as data is forwarded to its token. Discovering metrics, managing log sinks, and enriching resources additionally needs a linked GCP project (Workload Identity Federation validated against your project). Until a project is linked, Better Stack shows the "Connect your GCP project" step, even while ingestion works.

Link the project with a `logtail_source_gcp_project` resource. The project ID and project number live on that resource rather than on `logtail_source` itself: the Better Stack GCP Terraform module consumes the source token and the link consumes the module outputs, so keeping them separate keeps the dependency graph acyclic and lets the whole flow apply in one run.

## Single apply with the Better Stack GCP module

Create the source, let the [Better Stack GCP Terraform module](https://github.com/BetterStackHQ/gcp/tree/main/terraform) configure Workload Identity Federation, then link the project from its outputs (`logtail_source` then `module` then `logtail_source_gcp_project`):

```terraform
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
```

## When the project was set up out-of-band

If Workload Identity Federation was configured separately (the [setup script](https://github.com/BetterStackHQ/gcp), or the module applied elsewhere), pass the project ID and project number to the link resource directly. There is no cycle because both are static inputs:

```terraform
resource "logtail_source" "gcp" {
  name     = "GCP production"
  platform = "gcp"
}

resource "logtail_source_gcp_project" "gcp" {
  source_id          = logtail_source.gcp.id
  gcp_project_id     = var.betterstack_gcp_project_id
  gcp_project_number = var.betterstack_gcp_project_number
}
```

## Reusing an already-connected project

To attach a source to a project you have already connected, reference the connected account by ID:

```terraform
resource "logtail_source_gcp_project" "gcp" {
  source_id      = logtail_source.gcp.id
  gcp_account_id = "42"
}
```
