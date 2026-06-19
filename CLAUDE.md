# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository (the Better Stack Telemetry Terraform provider).

## Examples are both the docs and the E2E tests

Every resource and data source has an example under `examples/resources/<type>/resource.tf` or `examples/data-sources/<type>/data-source.tf`. `tfplugindocs` (`make gen`) injects them into the registry docs, and the `e2e_combined` CI job flattens them all into one configuration — the basic example's scaffolding minus its `main.tf`/`outputs.tf`, plus every example except the denylisted ones — and runs `apply` → empty `plan` → `destroy` against the live team. Add or extend an example for any new capability; one that appears in no example is never covered end-to-end.

Each example holds only its own resource and may reference siblings by their conventional name (e.g. `logtail_dashboard.production`); the union must be one valid config. Data sources must resolve by a **unique** key — a config resource's `id`/`table_name`, or a permanent uniquely-named fixture seeded in the E2E team — never a duplicate-prone name (a name shared with a freshly-created resource deadlocks `destroy` once runs overlap or leave leftovers).

`examples/` itself is the runnable **basic** example. `examples/connection/` runs as its own E2E config because it needs a global API token; the combined run denylists it and `logtail_source_aws_account` (needs AWS credentials).

**Seeded fixtures** the data-source examples look up (create once in the E2E team, never delete): collector `My Existing Collector`, `My Existing Source Group`, errors application `My Existing Errors Application` and group `My Existing Errors Application Group`, `My Existing Dashboard Group`, `My Existing Exploration`, `My Existing Exploration Group`; plus escalation policy `My Existing Escalation Policy` (used by the `exploration_alert` resource).

## Versioning: bump `VERSION` to the intended release version

**How releases work:** publishing to the Terraform registry is **solely tag-based** — pushing a `vX.Y.Z` git tag fires `.github/workflows/release.yml`, which builds and publishes that version. In practice the tag is pushed onto the squash-merged PR commit on master, so the PR itself is what gets released. The `Makefile`'s `VERSION` plays no part in publishing, but it drives local `make terraform` and E2E: the provider is built at `VERSION`, then `terraform init` runs against each example's `versions.tf` constraint, so **a constraint ahead of `VERSION` fails `init`**.

**The rule:** in every PR, set `VERSION` in the `Makefile` to the **intended release version** — the version of the **next git tag** this PR should be released in, so always **higher than the latest git tag** (`git describe --tags --abbrev=0`; usually its next patch, a minor bump for bigger changes). Never derive it from the current `VERSION` value, which may be stale — it had drifted to `10.14.0` while `v10.14.1` was already tagged. The only exception is a change unrelated to a release, such as a CI, instructions, or tests-only update — those don't bump anything.

**When an example starts using a brand-new capability**, also raise `version = ">= X.Y.Z"` in `examples/versions.tf` to the same intended release version, in the same commit as the `Makefile` bump. Registry users on an older provider then get a clean "update your provider" error instead of a confusing "unsupported parameter" one — and E2E `init` stays green because the constraint never gets ahead of `VERSION`.
