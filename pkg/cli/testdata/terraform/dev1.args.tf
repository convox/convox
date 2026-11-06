		module "system" {
			source = "github.com/convox/convox//terraform/system/local?ref=foo"
			baz = "qux"
			foo = "bar"
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
