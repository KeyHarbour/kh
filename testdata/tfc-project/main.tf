# Simple resources to create state in Terraform Cloud
# This project is used for testing migration to KeyHarbour

resource "random_id" "example" {
  byte_length = 8
}

resource "null_resource" "example" {
  triggers = {
    random_id = random_id.example.hex
  }
}

resource "random_pet" "example" {
  length    = 2
  separator = "-"
}
