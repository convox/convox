		module "system" {
			source = "github.com/convox/convox//terraform/system/local?ref=foo"
			name = "dev1"
			release = "foo"
		}

		output "api" {
			value     = module.system.api
			sensitive = true
		}

		output "provider" {
			value = "local"
		}

		output "release" {
			value = "foo"
		}
