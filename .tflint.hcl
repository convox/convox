plugin "aws" {
  enabled = true
  version = "0.36.0"
  source  = "github.com/terraform-linters/tflint-ruleset-aws"
}

# Disable all built-in Terraform style/convention rules.
# The existing codebase predates these rules and enforcing them
# would require touching every .tf file. Only the AWS plugin
# rules remain — those catch real infrastructure mistakes like
# invalid resource types and deprecated attributes.
plugin "terraform" {
  enabled = false
}
