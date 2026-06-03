output "source_id" {
  description = "The Better Stack source ID."
  value       = logtail_source.aws.id
}

output "source_token" {
  description = "The source token used by the CloudFormation stack (Firehose forwarding)."
  value       = logtail_source.aws.token
  sensitive   = true
}

output "integration_role_arn" {
  description = "The IAM role ARN created by the CloudFormation stack."
  value       = aws_cloudformation_stack.better_stack.outputs["IntegrationRoleArn"]
}
