		module "system" {
			source = "github.com/convox/convox//terraform/system/local"

			name    = "dev1"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
