		module "system" {
			source = "github.com/convox/convox//terraform/system/local?ref=otherver"
			name = "dev1"
			release = "otherver"
		}

		output "api" {
			value     = module.system.api
			sensitive = true
		}

		output "provider" {
			value = "local"
		}

		output "release" {
			value = "otherver"
		}
