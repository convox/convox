		module "system" {
			source = "github.com/convox/convox//terraform/system/local"

			name = "dev1"
			baz = "qux"
			foo = "bar"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
