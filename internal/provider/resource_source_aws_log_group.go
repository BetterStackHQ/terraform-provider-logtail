package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sourceAWSLogGroupSchema = map[string]*schema.Schema{
	"source_id": {
		Description: "The ID of the AWS `logtail_source` whose explicit log-group subscription this resource manages.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"region": {
		Description: "The AWS region containing the CloudWatch log group.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"name": {
		Description: "The CloudWatch log-group name.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"subscribed": {
		Description: "Whether Better Stack should subscribe this log group. Defaults to true.",
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     true,
	},
}

type sourceAWSLogGroup struct {
	Region     string `json:"region"`
	Name       string `json:"name"`
	Subscribed bool   `json:"subscribed"`
}

type sourceAWSLogGroupHTTPResponse struct {
	Data struct {
		ID         string            `json:"id"`
		Attributes sourceAWSLogGroup `json:"attributes"`
	} `json:"data"`
}

func newSourceAWSLogGroupResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceAWSLogGroupCreate,
		ReadContext:   sourceAWSLogGroupRead,
		UpdateContext: sourceAWSLogGroupUpdate,
		DeleteContext: sourceAWSLogGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: sourceAWSLogGroupImport,
		},
		Description: "Manages one explicit CloudWatch log-group subscription override for an AWS source. " +
			"The override may be created before AWS discovery finds the group. Destroying it returns the group to the source's automatic behavior.",
		Schema: sourceAWSLogGroupSchema,
	}
}

func sourceAWSLogGroupCollectionPath(d *schema.ResourceData) string {
	return fmt.Sprintf("/api/v2/sources/%s/aws-log-group-subscriptions", url.PathEscape(d.Get("source_id").(string)))
}

func sourceAWSLogGroupMemberPath(d *schema.ResourceData) string {
	return fmt.Sprintf("%s/%s", sourceAWSLogGroupCollectionPath(d), url.PathEscape(d.Id()))
}

func sourceAWSLogGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := sourceAWSLogGroup{
		Region:     d.Get("region").(string),
		Name:       d.Get("name").(string),
		Subscribed: d.Get("subscribed").(bool),
	}
	var out sourceAWSLogGroupHTTPResponse
	if derr := resourceCreate(ctx, meta, sourceAWSLogGroupCollectionPath(d), &in, &out); derr != nil {
		return derr
	}

	d.SetId(out.Data.ID)
	return sourceAWSLogGroupCopyAttrs(d, &out.Data.Attributes)
}

func sourceAWSLogGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceAWSLogGroupHTTPResponse
	if derr, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), sourceAWSLogGroupMemberPath(d), &out); derr != nil {
		return derr
	} else if !ok {
		d.SetId("")
		return nil
	}

	return sourceAWSLogGroupCopyAttrs(d, &out.Data.Attributes)
}

func sourceAWSLogGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	in := struct {
		Subscribed bool `json:"subscribed"`
	}{Subscribed: d.Get("subscribed").(bool)}
	if derr := resourceUpdate(ctx, meta, sourceAWSLogGroupMemberPath(d), &in); derr != nil {
		return derr
	}

	return sourceAWSLogGroupRead(ctx, d, meta)
}

func sourceAWSLogGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if derr := resourceDelete(ctx, meta, sourceAWSLogGroupMemberPath(d)); derr != nil {
		return derr
	}
	d.SetId("")
	return nil
}

// Import format: source_id/subscription_id.
func sourceAWSLogGroupImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid AWS log-group subscription ID format %q, expected 'source_id/subscription_id'", d.Id())
	}
	if err := d.Set("source_id", parts[0]); err != nil {
		return nil, err
	}
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}

func sourceAWSLogGroupCopyAttrs(d *schema.ResourceData, in *sourceAWSLogGroup) diag.Diagnostics {
	if err := d.Set("region", in.Region); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", in.Name); err != nil {
		return diag.FromErr(err)
	}
	return diag.FromErr(d.Set("subscribed", in.Subscribed))
}
