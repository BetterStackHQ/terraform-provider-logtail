package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var sourceAWSAccountSchema = map[string]*schema.Schema{
	"source_id": {
		Description: "The ID of the `logtail_source` (with `platform = \"aws\"`) to link the AWS account to. Changing this forces a new resource, re-running the account connect against the new source.",
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
	},
	"auto_subscribe_log_groups": {
		Description: "Whether Better Stack automatically subscribes CloudWatch log groups without an explicit override. When omitted, the current API setting is preserved.",
		Type:        schema.TypeBool,
		Optional:    true,
		Computed:    true,
	},
	"aws_account_id": {
		Description:  "The ID of an existing connected AWS account to link this source to. Provide this instead of `aws_role_arn`/`aws_external_id` to reuse an account you've already connected. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		AtLeastOneOf: []string{"aws_account_id", "aws_role_arn"},
	},
	"aws_role_arn": {
		Description:  "The IAM role ARN to connect your AWS account - the `IntegrationRoleArn` output of the Better Stack CloudFormation stack. Provide together with `aws_external_id`. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		RequiredWith: []string{"aws_external_id"},
	},
	"aws_external_id": {
		Description:  "The external ID used for the STS assume-role trust - the `ExternalId` output of the Better Stack CloudFormation stack. Provide together with `aws_role_arn`. Write-only: the API does not return it, so it isn't refreshed from state.",
		Type:         schema.TypeString,
		Optional:     true,
		Sensitive:    true,
		RequiredWith: []string{"aws_role_arn"},
	},
}

func newSourceAWSAccountResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: sourceAWSAccountCreate,
		ReadContext:   sourceAWSAccountRead,
		UpdateContext: sourceAWSAccountUpdate,
		DeleteContext: sourceAWSAccountDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Links an AWS account to an `aws` platform `logtail_source` by pasting back the " +
			"CloudFormation role ARN / external ID (or reusing an already-connected account).\n\n" +
			"The credentials are write-only - the API never returns them, so they aren't refreshed into " +
			"Terraform state. Destroying this resource only removes it from state; the AWS account stays " +
			"linked to the source until the source itself is destroyed.",
		Schema: sourceAWSAccountSchema,
	}
}

// sourceAWSAccountPayload is the PATCH body for account linkage and its log-group default.
// Credentials are write-only; the auto-subscribe setting is returned by the Sources API.
type sourceAWSAccountPayload struct {
	AwsAccountID        *string `json:"aws_account_id,omitempty"`
	AwsRoleArn          *string `json:"aws_role_arn,omitempty"`
	AwsExternalID       *string `json:"aws_external_id,omitempty"`
	AWSAutoSubLogGroups *bool   `json:"aws_auto_sub_log_groups,omitempty"`
}

// patchSourceAWSAccount sends the configured account settings. The full credential set is
// always sent because the connect manager rejects a partial role ARN / external ID pair.
func patchSourceAWSAccount(ctx context.Context, d *schema.ResourceData, meta interface{}, sourceID string) diag.Diagnostics {
	in := sourceAWSAccountPayload{
		AwsAccountID:        stringFromResourceData(d, "aws_account_id"),
		AwsRoleArn:          stringFromResourceData(d, "aws_role_arn"),
		AwsExternalID:       stringFromResourceData(d, "aws_external_id"),
		AWSAutoSubLogGroups: boolFromResourceData(d, "auto_subscribe_log_groups"),
	}
	return resourceUpdate(ctx, meta, fmt.Sprintf("/api/v2/sources/%s", url.PathEscape(sourceID)), &in)
}

func sourceAWSAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	sourceID := d.Get("source_id").(string)
	if derr := patchSourceAWSAccount(ctx, d, meta, sourceID); derr != nil {
		return derr
	}
	d.SetId(sourceID)
	return sourceAWSAccountRead(ctx, d, meta)
}

func sourceAWSAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var out sourceHTTPResponse
	if err, ok := resourceReadWithBaseURL(ctx, meta, meta.(*client).TelemetryBaseURL(), fmt.Sprintf("/api/v2/sources/%s", url.PathEscape(d.Id())), &out); err != nil {
		return err
	} else if !ok {
		d.SetId("") // Source gone -> linkage gone.
		return nil
	}
	// Credentials are write-only; refresh the readable setting and confirm the source still exists.
	if out.Data.Attributes.AWSAutoSubLogGroups != nil {
		if err := d.Set("auto_subscribe_log_groups", *out.Data.Attributes.AWSAutoSubLogGroups); err != nil {
			return diag.FromErr(err)
		}
	}
	return diag.FromErr(d.Set("source_id", d.Id()))
}

func sourceAWSAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if derr := patchSourceAWSAccount(ctx, d, meta, d.Get("source_id").(string)); derr != nil {
		return derr
	}
	return sourceAWSAccountRead(ctx, d, meta)
}

func sourceAWSAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The Sources API has no disconnect endpoint: PATCHing blank AWS params is a server-side
	// no-op (the connect manager never unlinks an account). Deleting this resource therefore
	// only drops it from Terraform state; the AWS account stays linked to the source until the
	// source itself is destroyed.
	d.SetId("")
	return nil
}
