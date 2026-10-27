package provider

// The API stores a multi_select_with_sql variable as select_with_sql with allow_multiple_values
// and returns it that way. The configuration keeps the multi_select_with_sql spelling, so the
// type is translated on the wire; the flag is neither sent nor read for the other types.

// apiVariableType is the variable_type and allow_multiple_values to send for a configured type.
func apiVariableType(configured string) (*string, *bool) {
	switch configured {
	case "multi_select_with_sql":
		return stringPtr("select_with_sql"), boolPtr(true)
	case "select_with_sql":
		return stringPtr(configured), boolPtr(false)
	default:
		return stringPtr(configured), nil
	}
}

// configuredVariableType is the spelling of a variable the API returned.
func configuredVariableType(apiType string, allowMultipleValues *bool) string {
	if apiType == "select_with_sql" && allowMultipleValues != nil && *allowMultipleValues {
		return "multi_select_with_sql"
	}
	return apiType
}
