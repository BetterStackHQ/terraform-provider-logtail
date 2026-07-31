package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// NullableInt models an integer API field whose null is a meaningful value (e.g. a
// null recovery_period is the Never recovery mode), represented in Terraform by a
// sentinel such as -1, since HCL cannot distinguish null from an unset attribute.
// Ported from terraform-provider-better-uptime, where it backs ssl_expiration and
// domain_expiration.
//
//   - Unset: the field is omitted from JSON (nil *NullableInt with omitempty)
//   - Set to the sentinel: the field is sent as JSON null (ExplicitNull)
//   - Set to a value: the field is sent as that value
type NullableInt struct {
	Value        *int
	ExplicitNull bool
}

func (n NullableInt) MarshalJSON() ([]byte, error) {
	if n.ExplicitNull {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

func (n *NullableInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Value = nil
		n.ExplicitNull = true
		return nil
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.Value = &v
	n.ExplicitNull = false
	return nil
}

// NullableIntFromResourceData loads a nullable int field from the configuration:
// nil when the field is not set in config (omit from the request), ExplicitNull when
// it is set to terraformNullValue, and the value otherwise. Built on
// intFromResourceData so an explicit 0 is preserved.
func NullableIntFromResourceData(d *schema.ResourceData, key string, terraformNullValue int) *NullableInt {
	v := intFromResourceData(d, key)
	if v == nil {
		return nil
	}
	if *v == terraformNullValue {
		return &NullableInt{ExplicitNull: true}
	}
	return &NullableInt{Value: v}
}

// SetNullableIntResourceData writes an API value into state, mapping both an omitted
// field (nil) and an explicit null to terraformNullValue. Only use it for fields the
// API always returns, where a missing key can't mean anything but null.
func SetNullableIntResourceData(d *schema.ResourceData, key string, terraformNullValue int, nullableInt *NullableInt) error {
	if nullableInt == nil || nullableInt.ExplicitNull {
		return d.Set(key, terraformNullValue)
	}
	return d.Set(key, *nullableInt.Value)
}
