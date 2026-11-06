		module "system" {
			source = "github.com/convox/convox//terraform/system/local"
			baz = "qux"
			name = "dev2"
			other = "side"
			release = ""
		}

		output "api" {
			value     = module.system.api
			sensitive = true
		}

		output "provider" {
			value = "local"
		}
