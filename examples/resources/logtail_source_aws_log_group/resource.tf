resource "logtail_source_aws_log_group" "application" {
  source_id  = logtail_source_aws_account.aws.source_id
  region     = "us-east-1"
  name       = "/aws/lambda/application"
  subscribed = true
}
