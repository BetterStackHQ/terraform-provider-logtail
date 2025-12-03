package provider

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// StringOrInt handles JSON fields that can be either a string or an integer.
// It unmarshals both "t1234" (string) and 123456 (number) into a string value,
// and marshals back to the appropriate JSON type based on the value.
//
// This is used for fields like team_id in the Better Stack API which can return:
// - A number: 12345
// - A string: "t1234"
//
// When marshaling:
// - Pure numeric strings like "1234" are marshaled as JSON numbers: 1234
// - Non-numeric strings like "t1234" are marshaled as JSON strings: "t1234"
//
// Example usage in resource_source.go:
//
//	type source struct {
//	    TeamId *StringOrInt `json:"team_id,omitempty"`
//	}
type StringOrInt string

// MarshalJSON implements json.Marshaler interface.
// Used when sending data to the Better Stack API:
// - If the value is a pure number (e.g., "1234"), marshals as JSON number: 1234
// - If the value contains non-numeric characters (e.g., "t1234"), marshals as JSON string: "t1234"
func (s StringOrInt) MarshalJSON() ([]byte, error) {
	str := string(s)
	// Try to parse as integer - if successful, marshal as number
	if n, err := strconv.ParseInt(str, 10, 64); err == nil {
		return json.Marshal(n)
	}
	// Otherwise marshal as string
	return json.Marshal(str)
}

// UnmarshalJSON implements json.Unmarshaler interface.
// Used when receiving data from the Better Stack API:
// - When the JSON value is a string (e.g., "t1234"), stores it as-is
// - When the JSON value is a number (e.g., 123456), converts it to a string
func (s *StringOrInt) UnmarshalJSON(data []byte) error {
	// Try string first (handles "t1234", "b654654")
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrInt(str)
		return nil
	}
	// Try number (handles 6547, 123456)
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*s = StringOrInt(n.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into StringOrInt", data)
}

// String returns the underlying string value.
func (s StringOrInt) String() string {
	return string(s)
}

// StringOrIntFromResourceData creates a StringOrInt from a Terraform resource field.
// Used in sourceCreate and sourceUpdate to handle fields that can be string or int.
//
// Parameters:
//   - d: The Terraform resource data
//   - key: The field name (e.g., "team_id")
//
// Example:
//
//	in.TeamId = StringOrIntFromResourceData(d, "team_id")
func StringOrIntFromResourceData(d *schema.ResourceData, key string) *StringOrInt {
	if v, ok := d.GetOk(key); ok {
		if v == nil {
			return nil
		}
		str := v.(string)
		result := StringOrInt(str)
		return &result
	}
	return nil
}

// SetStringOrIntResourceData sets a Terraform resource field from a StringOrInt.
// Used in sourceCopyAttrs to handle fields that can be string or int.
//
// Parameters:
//   - d: The Terraform resource data
//   - key: The field name (e.g., "team_id")
//   - value: The StringOrInt value from the API
//
// Example:
//
//	SetStringOrIntResourceData(d, "team_id", in.TeamId)
func SetStringOrIntResourceData(d *schema.ResourceData, key string, value *StringOrInt) error {
	if value == nil {
		return d.Set(key, nil)
	}
	return d.Set(key, string(*value))
}
