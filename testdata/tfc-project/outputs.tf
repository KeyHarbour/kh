output "random_id" {
  description = "Random ID for testing"
  value       = random_id.example.hex
}

output "random_pet" {
  description = "Random pet name for testing"
  value       = random_pet.example.id
}

output "null_resource_id" {
  description = "Null resource ID"
  value       = null_resource.example.id
}
