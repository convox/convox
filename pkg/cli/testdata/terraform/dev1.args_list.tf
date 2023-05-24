
		module "system" {
			source = "github.com/convox/convox//terraform/system/local?ref=foo"
			foo = ["bar", "bar", "zzz", ]
			name = "dev1"
			release = "foo"
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "local"
		}

		output "release" {
			value = "foo"
		}
