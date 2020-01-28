		module "system" {
			source = "github.com/convox/convox//terraform/system/local"

			name = "dev2"
			baz = "qux"
			other = "side"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
