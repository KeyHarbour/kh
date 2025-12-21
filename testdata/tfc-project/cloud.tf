terraform {
  cloud {
    organization = "KeyHarbour"

    workspaces {
      tags = ["cli-test"]
    }
  }
}
