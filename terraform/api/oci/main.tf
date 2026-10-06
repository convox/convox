module "elasticsearch" {
  source = "../../elasticsearch/k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
  replicas  = var.high_availability ? 2 : 1
}

module "fluentd" {
  source = "../../fluentd/elasticsearch"

  providers = {
    kubernetes = kubernetes
  }

  cluster        = var.cluster
  elasticsearch  = module.elasticsearch.host
  fluentd_memory = var.fluentd_memory
  namespace      = var.namespace
  rack           = var.name
  syslog         = var.syslog
}

module "k8s" {
  source = "../k8s"

  depends_on = [
    oci_objectstorage_bucket.storage
  ]

  providers = {
    kubernetes = kubernetes
  }

  buildkit_enabled          = var.buildkit_enabled
  docker_hub_authentication = var.docker_hub_authentication
  domain                    = var.domain
  image                     = var.image
  namespace                 = var.namespace
  rack                      = var.name
  rack_name                 = var.rack_name
  release                   = var.release
  resolver                  = var.resolver
  replicas                  = var.high_availability ? 2 : 1
  webhook_signing_key       = var.webhook_signing_key

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
  }

  env = {
    BUCKET            = oci_objectstorage_bucket.storage.name                                                                              # object storage bucket holding builds, releases and logs
    CERT_MANAGER      = "true"                                                                                                             # certificates come from cert-manager annotations
    COMPARTMENT       = var.compartment_ocid                                                                                               # compartment every rack resource lives in
    ELASTIC_URL       = module.elasticsearch.url                                                                                           # in cluster elasticsearch for logs
    PROVIDER          = "oci"                                                                                                              # selects the oci provider implementation
    REGION            = var.region                                                                                                         # oci region name, eg us-ashburn-1
    REGISTRY          = "${local.region_key}.ocir.io/${data.oci_objectstorage_namespace.current.namespace}/${var.name}"                    # ocir base path; the api appends /<app> to make the repo
    REGISTRY_PASSWORD = oci_identity_auth_token.api.token                                                                                  # ocir docker password (iam auth token)
    REGISTRY_USERNAME = "${data.oci_objectstorage_namespace.current.namespace}/${oci_identity_user.api.name}"                              # ocir docker username
    RESOLVER          = var.resolver                                                                                                       # in cluster dns resolver endpoint
    ROUTER            = var.router                                                                                                         # rack router hostname
    S3_ACCESS         = oci_identity_customer_secret_key.api.id                                                                            # s3 compatibility access key id
    S3_ENDPOINT       = "https://${data.oci_objectstorage_namespace.current.namespace}.compat.objectstorage.${var.region}.oraclecloud.com" # s3 compatibility endpoint for BUCKET
    S3_SECRET         = oci_identity_customer_secret_key.api.key                                                                           # s3 compatibility secret access key
  }
}
