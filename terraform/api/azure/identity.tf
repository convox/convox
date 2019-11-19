# resource "azurerm_user_assigned_identity" "api" {
#   resource_group_name = data.azurerm_resource_group.rack.name
#   location            = data.azurerm_resource_group.rack.location

#   name = "api"
# }

# resource "azurerm_role_assignment" "identity-api-contributor" {
#   scope                = data.azurerm_resource_group.rack.id
#   role_definition_name = "Contributor"
#   principal_id         = azurerm_user_assigned_identity.api.principal_id
# }

# data "template_file" "identity" {
#   template = file("${path.module}/identity.yml.tpl")

#   vars = {
#     namespace = var.namespace
#     resource  = azurerm_user_assigned_identity.api.id
#     client    = azurerm_user_assigned_identity.api.client_id
#   }
# }

# resource "null_resource" "deployment" {
#   provisioner "local-exec" {
#     when    = "create"
#     command = "echo '${data.template_file.identity.rendered}' | kubectl apply -f -"
#     environment = {
#       "KUBECONFIG" : var.kubeconfig,
#     }
#   }

#   provisioner "local-exec" {
#     when    = "destroy"
#     command = "echo '${data.template_file.identity.rendered}' | kubectl delete -f -"
#     environment = {
#       "KUBECONFIG" : var.kubeconfig,
#     }
#   }

#   triggers = {
#     template = sha256(data.template_file.identity.rendered)
#   }
# }
