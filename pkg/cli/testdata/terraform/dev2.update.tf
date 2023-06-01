		module "system" {
			source = "github.com/convox/convox//terraform/system/local"
			baz = "qux"
			name = "dev2"
			other = "side"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
