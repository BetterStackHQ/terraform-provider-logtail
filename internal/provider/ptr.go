package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
