package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newMetricDataSource() *schema.Resource {
	s := make(map[string]*schema.Schema)
	for k, v := range metricSchema {
		cp := *v
		switch k {
		case "source_id":
		case "name":
			cp.Computed = false
			cp.Optional = false
			cp.Required = true
		default:
			cp.Computed = true
			cp.Optional = false
			cp.Required = false
			cp.ValidateFunc = nil
			cp.ValidateDiagFunc = nil
			cp.Default = nil
			cp.DefaultFunc = nil
			cp.DiffSuppressFunc = nil
		}
		s[k] = &cp
	}
	return &schema.Resource{
		ReadContext: metricLookup,
		Description: "This Data Source allows you to look up existing Metrics using their name. You can list all your existing metrics via the [Metrics API](https://betterstack.com/docs/logs/api/list-all-existing-metrics/).",
		Schema:      s,
	}
}
