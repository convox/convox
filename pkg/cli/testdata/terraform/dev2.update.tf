		module "system" {
			source = "github.com/convox/convox//terraform/system/local"
			baz = "qux"
			name = "dev2"
			other = "side"
			rack_name = "dev2"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
