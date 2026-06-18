# Combined examples (end-to-end test)

This directory turns every per-resource example under
[`examples/resources/`](../resources) into a **single Terraform configuration**,
so the whole set is exercised by one `apply` → `plan` (must be empty) → `destroy`
in CI. That keeps the resource snippets shown in the registry docs continuously
verified against the live API.

## How it works

- `make combine` runs [`combine.sh`](./combine.sh), which copies each example's
  `*.tf` into this directory as `gen_<dir>__<file>` (gitignored). Terraform reads
  all `.tf` files in a directory as one configuration.
- Each example holds only its own resource and references siblings by their
  conventional name (e.g. `logtail_dashboard.production`), so the union resolves
  without collisions.
- The provider block, version constraints, and shared variables live in the
  committed `provider.tf`, `versions.tf`, and `variables.tf`.
- [`skip.txt`](./skip.txt) lists resource directories that stay in the docs but
  cannot run in CI (e.g. AWS credentials or a global API token are required).
- Data-source examples (`examples/data-sources/`) are **not** assembled here: they
  look resources up by name, which is ambiguous — and blocks `destroy` — once names
  duplicate across parallel or retried runs. They stay in the docs, just not executed.

## Run it locally

```shell
make combine
make terraform CONFIGURATION=examples/all ARGS="apply --auto-approve"
make terraform CONFIGURATION=examples/all ARGS="destroy --auto-approve"
```
