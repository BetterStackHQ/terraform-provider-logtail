output "connection_host" {
  description = "Connection hostname"
  value       = logtail_connection.example.host
}

output "connection_username" {
  description = "Connection username"
  value       = logtail_connection.example.username
}

output "connection_password" {
  description = "Connection password"
  value       = logtail_connection.example.password
  sensitive   = true
}

output "data_sources" {
  description = "Data sources of the connection"
  value       = logtail_connection.example.data_sources
}
