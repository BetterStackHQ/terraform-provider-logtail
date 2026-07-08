package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// intFromResourceData returns a pointer to an int if the field was explicitly set in the config.
// This allows sending 0 values to the API while omitting unset fields.
func intFromResourceData(d *schema.ResourceData, key string) *int {
	rawConfig := d.GetRawConfig()
	if rawConfig.IsNull() || !rawConfig.IsKnown() {
		return nil
	}
	val := rawConfig.GetAttr(key)
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	v := d.Get(key).(int)
	return &v
}

// floatFromResourceData returns a pointer to a float64 if the field was explicitly set in the config.
// This allows sending 0 values to the API while omitting unset fields.
func floatFromResourceData(d *schema.ResourceData, key string) *float64 {
	rawConfig := d.GetRawConfig()
	if rawConfig.IsNull() || !rawConfig.IsKnown() {
		return nil
	}
	val := rawConfig.GetAttr(key)
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	v := d.Get(key).(float64)
	return &v
}

// boolFromResourceData returns a pointer to a bool if the field was explicitly set in the config.
// This allows sending false values to the API while omitting unset fields.
func boolFromResourceData(d *schema.ResourceData, key string) *bool {
	rawConfig := d.GetRawConfig()
	if rawConfig.IsNull() || !rawConfig.IsKnown() {
		return nil
	}
	val := rawConfig.GetAttr(key)
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	v := d.Get(key).(bool)
	return &v
}

// stringFromResourceData returns a pointer to a string only if the field was explicitly
// set in the config. Used for write-only fields (e.g. the AWS account linkage params) that
// the API never returns, so "configured" must be distinguished from "unset" without relying
// on read-back.
//
// The value is read directly from rawConfig because d.Get returns the (stale) state value
// for Computed attributes when Terraform's diff logic treats an explicit "" the same as unset.
func stringFromResourceData(d *schema.ResourceData, key string) *string {
	rawConfig := d.GetRawConfig()
	if rawConfig.IsNull() || !rawConfig.IsKnown() {
		return nil
	}
	val := rawConfig.GetAttr(key)
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	v := val.AsString()
	return &v
}

// nolint
func load(d *schema.ResourceData, key string, receiver interface{}) {
	switch x := receiver.(type) {
	case **string:
		if v, ok := d.GetOkExists(key); ok {
			t := v.(string)
			*x = &t
		}
	case **int:
		if v, ok := d.GetOkExists(key); ok {
			t := v.(int)
			*x = &t
		}
	case **bool:
		if v, ok := d.GetOkExists(key); ok {
			t := v.(bool)
			*x = &t
		}
	case **[]string:
		if v, ok := d.GetOkExists(key); ok {
			var t []string
			for _, v := range v.([]interface{}) {
				t = append(t, v.(string))
			}
			*x = &t
		}
	case **[]int:
		if v, ok := d.GetOkExists(key); ok {
			var t []int
			for _, v := range v.([]interface{}) {
				t = append(t, v.(int))
			}
			*x = &t
		}
	case **[]map[string]interface{}:
		if v, ok := d.GetOkExists(key); ok {
			var t []map[string]interface{}
			for _, v := range v.([]interface{}) {
				entry := v.(map[string]interface{})
				newEntry := map[string]interface{}{}
				for mapKey, mapValue := range entry {
					newEntry[mapKey] = mapValue
				}
				t = append(t, newEntry)
			}
			*x = &t
		}

	default:
		panic(fmt.Errorf("unexpected type %T", receiver))
	}
}

// dropUnknownKeys removes keys not defined in the schema from raw API maps,
// so fields newly added to the API are ignored instead of failing d.Set.
func dropUnknownKeys(in *[]map[string]interface{}, s map[string]*schema.Schema) *[]map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make([]map[string]interface{}, len(*in))
	for i, m := range *in {
		filtered := make(map[string]interface{}, len(m))
		for k, v := range m {
			if _, ok := s[k]; ok {
				filtered[k] = v
			}
		}
		out[i] = filtered
	}
	return &out
}
