provider "logtail" {
  api_token = var.logtail_api_token
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
parsed, err = parse_regex(..message, r'\[svc:(?P<service>[a-zA-Z_-]+)\] .* in (?P<duration>\d+(?:\.\d+)?)ms')
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
          name           = "source"
          variable_type  = "source"
          values         = []
          default_values = null
          sql_definition = null
        },
        {
          name           = "level"
          variable_type  = "select_with_sql"
          values         = []
          default_values = null
          sql_definition = "level"
        },
        {
          name           = "start_time"
          variable_type  = "datetime"
          values         = ["now-3h"]
          default_values = null
          sql_definition = null
        },
        {
          name           = "end_time"
          variable_type  = "datetime"
          values         = ["now"]
          default_values = null
          sql_definition = null
        },
      ]
    }
    charts = [
      {
        chart_type     = "line_chart"
        name           = "Number of logs"
        description    = null
        x              = 0
        y              = 0
        w              = 9
        h              = 8
        transform_with = "// Transform chart data before rendering.\n// Following function is called when new data arrives, and again with `completed = true` after all data arrives.\n// You can transform the data here arbitrarily.\n// Most chart types expect columns 'time', 'value' and optionally 'series' by default.\nasync (existingDataByQuery, newDataByQuery, completed) => {\n  return Object.keys(newDataByQuery).reduce((result, queryIndex) => {\n    result[queryIndex] = result[queryIndex].concat(newDataByQuery[queryIndex]);\n    return result;\n  }, existingDataByQuery);\n}\n"
        finalize_with  = null
        fake_with      = null
        settings = {
          unit         = "shortened"
          label        = "shown_below"
          legend       = "shown_below"
          stacking     = "dont_stack"
          lat_column   = "latitude"
          lng_column   = "longitude"
          time_column  = "time"
          x_axis_type  = "time"
          y_axis_scale = "linear"
          series_colors = {
            value = "#009fe3"
          }
          series_column        = "series"
          value_columns        = ["value"]
          decimal_places       = 2
          point_size_column    = "size"
          treat_missing_values = "connected"
          guessed_series_colors = {
            value = "#009fe3"
          }
        }
        chart_queries = [
          {
            name            = null
            query_type      = "sql_expression"
            sql_query       = "SELECT {{time}} as time, countMerge(events_count) as value\nFROM {{source}}\nWHERE time BETWEEN {{start_time}} AND {{end_time}}\n [[ AND level = {{level}} ]]\nGROUP BY time\n"
            where_condition = null
            static_text     = null
            y_axis = [
              {
                type    = "integer"
                value   = "events"
                measure = "count"
              }
            ]
            filters         = []
            group_by        = []
            source_variable = "source"
          }
        ]
        chart_alerts = []
      },
      {
        chart_type     = "static_text_chart"
        name           = "Static text"
        description    = null
        x              = 9
        y              = 0
        w              = 3
        h              = 8
        transform_with = "// Transform chart data before rendering.\n// Following function is called when new data arrives, and again with `completed = true` after all data arrives.\n// You can transform the data here arbitrarily.\n// Most chart types expect columns 'time', 'value' and optionally 'series' by default.\nasync (existingDataByQuery, newDataByQuery, completed) => {\n  return Object.keys(newDataByQuery).reduce((result, queryIndex) => {\n    result[queryIndex] = result[queryIndex].concat(newDataByQuery[queryIndex]);\n    return result;\n  }, existingDataByQuery);\n}\n"
        finalize_with  = null
        fake_with      = null
        settings = {
          unit         = "shortened"
          fresh        = true
          label        = "shown_below"
          legend       = "shown_below"
          stacking     = "dont_stack"
          lat_column   = "latitude"
          lng_column   = "longitude"
          time_column  = "time"
          x_axis_type  = "time"
          y_axis_scale = "linear"
          series_colors = {
            value = "#009fe3"
          }
          series_column        = "series"
          value_columns        = ["value"]
          decimal_places       = 2
          point_size_column    = "size"
          treat_missing_values = "connected"
          guessed_series_colors = {
            value = "#009fe3"
          }
        }
        chart_queries = [
          {
            name            = null
            query_type      = "static_text"
            sql_query       = null
            where_condition = null
            static_text     = "## Imported from Terraform\n\nThis is an example custom dashboard."
            y_axis = [
              {
                name    = "events"
                type    = "integer"
                value   = "events"
                measure = "count"
              }
            ]
            filters         = []
            group_by        = []
            source_variable = null
          }
        ]
        chart_alerts = []
      }
    ]
    sections = []
  })
}
