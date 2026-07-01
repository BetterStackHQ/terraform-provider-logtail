# Bar chart over a source, grouped by status
resource "logtail_exploration" "this" {
  name = "Requests by status"

  chart {
    chart_type = "bar_chart"
  }

  query {
    query_type      = "sql_expression"
    sql_query       = "SELECT {{time}} AS time, JSONExtractString(raw, 'status') AS series, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} GROUP BY time, series"
    source_variable = "source"
  }

  # `values` holds the source ID(s) the queries and any alerts run against
  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }
}

# Pie chart filed under a group, with a fixed date range and display settings
# plus a user-selectable filter variable
resource "logtail_exploration" "level_breakdown" {
  name                 = "Events by level"
  date_range_from      = "now-7d"
  date_range_to        = "now"
  exploration_group_id = logtail_exploration_group.this.id

  chart {
    chart_type  = "pie_chart"
    description = "Distribution of events by level"
    settings = jsonencode({
      unit           = "shortened"
      decimal_places = 0
      legend         = "shown_below"
      label          = "shown_below"
    })
  }

  query {
    query_type      = "sql_expression"
    sql_query       = "SELECT JSONExtractString(raw, 'level') AS series, count(*) AS value FROM {{source}} WHERE time BETWEEN {{start_time}} AND {{end_time}} [[ AND series = {{level}} ]] GROUP BY series"
    source_variable = "source"
  }

  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }

  variable {
    name           = "level"
    variable_type  = "select_with_sql"
    sql_definition = "JSONExtractString(raw, 'level')"
  }
}

# Live tail of error logs - where_condition filters the stream
resource "logtail_exploration" "live_errors" {
  name = "Live error logs"

  chart {
    chart_type  = "tail_chart"
    description = "Live view of error logs"
  }

  query {
    query_type      = "tail_query"
    where_condition = "level=error"
    source_variable = "source"
  }

  variable {
    name          = "source"
    variable_type = "source"
    values        = [logtail_source.this.id]
  }
}

# A static-text panel for notes and links, no query against a source
resource "logtail_exploration" "notes" {
  name = "Investigation notes"

  chart {
    chart_type = "static_text_chart"
  }

  query {
    query_type  = "static_text"
    static_text = "## Investigation notes\n\nLink runbooks and context for this view here."
  }
}
