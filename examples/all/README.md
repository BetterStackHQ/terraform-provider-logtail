# Combined examples (end-to-end test)

This directory turns every per-resource and per-data-source example under
[`examples/resources/`](../resources) and [`examples/data-sources/`](../data-sources)
into a **single Terraform configuration**, so the whole set is exercised by one
`apply` → `plan` (must be empty) → `destroy` in CI. That keeps the snippets shown
in the registry docs continuously verified against the live API.

## How it works

- `make combine` runs [`combine.sh`](./combine.sh), which copies each example's
  `*.tf` into this directory as `gen_<dir>__<file>` (gitignored). Terraform reads
  all `.tf` files in a directory as one configuration.
- Each resource example holds only its own resource and references siblings by
  their conventional name (e.g. `logtail_dashboard.production`), so the union
  resolves without collisions.
- Each data-source example resolves by a **unique key** — a config resource's
  `id`/`table_name` (or an id-scoped name), or a permanent uniquely-named fixture
  seeded in the E2E team (`"My Existing …"`) — never a duplicate-prone name, so
  concurrent or retried runs can't collide or deadlock. The fixtures the team must
  hold are listed in [`fixtures.md`](./fixtures.md).
- The provider block, version constraints, and shared variables live in the
  committed `provider.tf`, `versions.tf`, and `variables.tf`.
- [`skip.txt`](./skip.txt) lists directories that stay in the docs but cannot run
  in CI (e.g. AWS credentials or a global API token are required).

## Run it locally

```shell
make combine
make terraform CONFIGURATION=examples/all ARGS="apply --auto-approve"
make terraform CONFIGURATION=examples/all ARGS="destroy --auto-approve"
```
