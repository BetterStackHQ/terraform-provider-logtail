# E2E fixtures (permanent seeded resources)

The combined E2E (see [`README.md`](./README.md)) has each data-source example look up a
**permanent, uniquely-named resource** that lives in the E2E team independently of any
test run. These must exist for the `data.*` reads to resolve, and they are never created
or destroyed by the test — so they can never duplicate or deadlock a run.

Create them once in the E2E team and leave them in place. Names must match exactly.

| Resource type | Name | Notes |
| --- | --- | --- |
| `logtail_collector` | `My Existing Collector` | any platform (e.g. `docker`) |
| `logtail_source_group` | `My Existing Source Group` | |
| `logtail_errors_application` | `My Existing Errors Application` | any platform (e.g. `ruby_errors`) |
| `logtail_errors_application_group` | `My Existing Errors Application Group` | |
| `logtail_dashboard_group` | `My Existing Dashboard Group` | |
| `logtail_exploration` | `My Existing Exploration` | needs one chart + query to be valid |
| `logtail_exploration_group` | `My Existing Exploration Group` | |

The remaining data sources need **no fixture**: `source`, `metric`, `dashboard` and its
`dashboard_chart` / `dashboard_section` / `dashboard_alert`, and `exploration_alert` all
key off a config resource's unique `id` / `table_name`. `dashboard_template` reads the
built-in `"Hosts"` template, and `logtail_connection` is excluded (it needs a global token).

Already relied on by `examples/advanced`: an escalation policy named
`My Existing Escalation Policy`.
