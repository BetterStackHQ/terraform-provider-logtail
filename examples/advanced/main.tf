provider "logtail" {
  api_token = var.logtail_api_token
  # Defaults are already set to work nicely with Telemetry API
  # If needed, you can customize the configuration to better suit your use case
  api_retry_max      = 4
  api_retry_wait_min = 10
  api_retry_wait_max = 300
  api_timeout        = 60
  api_rate_limit     = 5
  api_rate_burst     = 10
}

resource "logtail_source_group" "group" {
  name = "Terraform Advanced Source Group"
}

resource "logtail_source" "this" {
  name               = "Terraform Advanced Source"
  platform           = "http"
  ingesting_paused   = true
  data_region        = "germany"
  live_tail_pattern  = "{level} {message}"
  logs_retention     = 60
  metrics_retention  = 90
  vrl_transformation = <<EOT
# Expected msg format: [svc:router] GET /api/health succeeded in 12.345ms
parsed, err = parse_regex(.message, r'\[svc:(?P<service>[a-zA-Z_-]+)\] .* in (?P<duration>\d+(?:\.\d+)?)ms')
if (err == null) {
    .service_name = parsed.service
    .duration_ms = to_float!(parsed.duration)
}
EOT
  source_group_id    = logtail_source_group.group.id
}

resource "logtail_metric" "duration_ms" {
  source_id      = logtail_source.this.id
  name           = "duration_ms"
  sql_expression = "getJSON(raw, 'duration_ms')"
  aggregations   = ["avg", "max", "min"]
  type           = "float64_delta"
}

resource "logtail_metric" "service_name" {
  source_id      = logtail_source.this.id
  name           = "service_name"
  sql_expression = "getJSON(raw, 'service_name')"
  aggregations   = []
  type           = "string_low_cardinality"
}

data "logtail_metric" "level" {
  source_id = logtail_source.this.id
  name      = "level"
}

resource "logtail_errors_application_group" "errors_group" {
  name = "Terraform Advanced Errors Application Group"
}

resource "logtail_errors_application" "this" {
  name                 = "Terraform Advanced Errors Application"
  platform             = "ruby_errors"
  ingesting_paused     = true
  data_region          = "germany"
  errors_retention     = 90
  application_group_id = logtail_errors_application_group.errors_group.id
}

data "logtail_errors_application" "lookup" {
  name = logtail_errors_application.this.name
}

# Find existing dashboard by name
data "logtail_dashboard" "host_prometheus" {
  name = "Host (Prometheus)"
  # Or by ID:
  # id = 1234
}

# Create a dashboard from a dashboard template
data "logtail_dashboard_template" "host_overview" {
  name = "Host Overview"
}
resource "logtail_dashboard" "from_template" {
  name = "My copy of Host overview"
  data = data.logtail_dashboard_template.host_overview.data
}

resource "logtail_dashboard" "custom" {
  name = "Terraform Custom Dashboard"
  data = jsonencode({
    refresh_interval = 0
    date_range_from  = "now-3h"
    date_range_to    = "now"
    preset = {
      preset_type = "implicit"
      preset_variables = [
        {
          name           = "level"
          variable_type  = "select_with_sql"
          values         = []
          default_values = null
          sql_definition = "level"
        },
      ]
    }
    charts = [
      {
        chart_type = "line_chart"
        name       = "Number of logs"
        x          = 0
        y          = 0
        w          = 9
        h          = 8
        settings = {
          unit        = "shortened"
          label       = "shown_below"
          legend      = "shown_below"
          stacking    = "dont_stack"
          time_column = "time"
          series_colors = {
            value = "#009fe3"
          }
          series_column        = "series"
          value_columns        = ["value"]
          decimal_places       = 2
          treat_missing_values = "connected"
        }
        chart_queries = [
          {
            query_type      = "sql_expression"
            sql_query       = "SELECT {{time}} as time, countMerge(events_count) as value\nFROM {{source}}\nWHERE time BETWEEN {{start_time}} AND {{end_time}}\n [[ AND level = {{level}} ]]\nGROUP BY time\n"
            source_variable = "source"
          }
        ]
        chart_alerts = []
      },
      {
        chart_type = "static_text_chart"
        name       = "Static text"
        x          = 9
        y          = 0
        w          = 3
        h          = 8
        chart_queries = [
          {
            query_type  = "static_text"
            static_text = "## Imported from Terraform\n\nThis is an example custom dashboard."
          }
        ]
        chart_alerts = []
      }
    ]
    sections = []
  })
}
