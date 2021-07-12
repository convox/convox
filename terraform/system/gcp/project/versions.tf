terraform {
  required_providers {
    google-direct = {
      source  = "hashicorp/google"
      version = "~> 3.74.0"
      # configuration_aliases = [ google.direct ]
    }
  }
  required_version = ">= 0.12"
}
