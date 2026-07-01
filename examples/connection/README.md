This directory contains a sample Terraform configuration for a ClickHouse connection - a SQL
endpoint you can point external tools at to query your ingested data - together with the source
it reads from.

Managing connections requires a **global** API token (not a team token), so this example sets
the team explicitly via `logtail_team_name`.

## Usage

```shell script
git clone https://github.com/BetterStackHQ/terraform-provider-logtail && \
  cd terraform-provider-logtail/examples/connection

echo '# See variables.tf for more. Requires a GLOBAL API token.
logtail_api_token = "XXXXXXXXXXXXXXXXXXXXXXXX"
logtail_team_name = "My Team"
' > terraform.tfvars

terraform init
terraform apply

# Connection details for your ClickHouse client:
terraform output connection_host
terraform output connection_username
terraform output -raw connection_password
```
