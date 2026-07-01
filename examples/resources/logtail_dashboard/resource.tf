# The hub dashboard - charts, sections and alerts attach to it via its ID
# and reference its `source` variable through {{source}}
resource "logtail_dashboard" "production" {
  name = "Production overview"

  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }
}

# Filed under a group, with a default date range and auto-refresh
resource "logtail_dashboard" "tuned" {
  name               = "Tuned dashboard"
  dashboard_group_id = logtail_dashboard_group.production.id
  date_range_from    = "now-24h"
  date_range_to      = "now"
  refresh_interval   = 60

  # Probe query run per source - sources where it returns no rows are flagged as not eligible for this dashboard
  source_eligibility_sql = "SELECT 1 FROM {{source}} WHERE label('level') != '' LIMIT 1"

  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }

  # Static dropdown - default_values are the options, values is the current pick
  variable {
    name           = "environment"
    variable_type  = "select_value"
    default_values = ["production", "staging", "development"]
    values         = ["production"]
  }

  # SQL-driven dropdown - options are the distinct values of the expression
  variable {
    name           = "level"
    variable_type  = "select_with_sql"
    sql_definition = "label('level')"
  }
}

# Clone a Better Stack dashboard template by passing its data through
resource "logtail_dashboard" "from_template" {
  name               = "Hosts (from template)"
  dashboard_group_id = logtail_dashboard_group.production.id
  data               = data.logtail_dashboard_template.hosts.data
}

# Import mode - the whole dashboard as one JSON blob, recreated on any change
# cannot be combined with chart/variable blocks or the tuning fields above
resource "logtail_dashboard" "imported" {
  name               = "Imported dashboard"
  dashboard_group_id = logtail_dashboard_group.production.id
  data = jsonencode({
    refresh_interval = 0
    date_range_from  = "now-3h"
    date_range_to    = "now"
    preset = {
      preset_type = "implicit"
      preset_variables = [
        {
          name          = "source"
          variable_type = "source"
          values        = [logtail_source.this.id]
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
          unit          = "shortened"
          time_column   = "time"
          series_column = "series"
          value_columns = ["value"]
        }
        chart_queries = [
          {
            query_type      = "sql_expression"
            sql_query       = "SELECT {{time}} as time, count(*) as value\nFROM {{source}}\nWHERE time BETWEEN {{start_time}} AND {{end_time}}\nGROUP BY time\n"
            source_variable = "source"
          }
        ]
        chart_alerts = []
      },
      {
        chart_type = "static_text_chart"
        name       = "Notes"
        x          = 9
        y          = 0
        w          = 3
        h          = 8
        chart_queries = [
          {
            query_type  = "static_text"
            static_text = "## Imported from Terraform"
          }
        ]
        chart_alerts = []
      }
    ]
    sections = []
  })
}
