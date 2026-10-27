package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The API stores a multi_select_with_sql variable as select_with_sql with allow_multiple_values
// and reports it that way, so a variable configured in that legacy spelling sits in state in the
// documented form. These two suppressions keep the spelling from showing up as a change on every
// plan; an update then resends the variable as the API stores it.

// suppressLegacyMultiSelectVariableType treats a configured multi_select_with_sql as equal to the
// select_with_sql in state as long as the state's allow_multiple_values is set.
func suppressLegacyMultiSelectVariableType(k, old, new string, d *schema.ResourceData) bool {
	if old != "select_with_sql" || new != "multi_select_with_sql" {
		return false
	}
	stateFlag, _ := d.GetChange(strings.TrimSuffix(k, "variable_type") + "allow_multiple_values")
	return stateFlag == true
}

// suppressAllowMultipleValuesOfLegacyType ignores the flag of a multi_select_with_sql variable:
// the type implies it, whatever the configuration says.
func suppressAllowMultipleValuesOfLegacyType(k, old, new string, d *schema.ResourceData) bool {
	return d.Get(strings.TrimSuffix(k, "allow_multiple_values")+"variable_type") == "multi_select_with_sql"
}
