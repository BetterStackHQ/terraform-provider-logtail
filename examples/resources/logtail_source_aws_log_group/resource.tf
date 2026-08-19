resource "logtail_source_aws_log_group" "application" {
  source_id  = logtail_source.aws.id
  region     = "us-east-1"
  name       = "/aws/lambda/application"
  subscribed = true
}

resource "logtail_source_aws_log_group" "excluded" {
  source_id  = logtail_source.aws.id
  region     = "us-east-1"
  name       = "/aws/lambda/noisy"
  subscribed = false
}
