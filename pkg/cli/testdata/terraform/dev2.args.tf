		module "system" {
			source = "github.com/convox/convox//terraform/system/local"
			baz = "qux"
			foo = "bar"
			name = "dev2"
			release = ""
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}
