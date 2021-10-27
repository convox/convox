terraform {
  required_providers {
    google = {
      configuration_aliases = [ google.direct ]
      source  = "hashicorp/google"
      version = "~> 3.5.0"
    }
  }
  required_version = ">= 0.12"
}
