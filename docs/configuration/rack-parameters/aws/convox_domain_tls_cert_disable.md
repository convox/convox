---
title: "convox_domain_tls_cert_disable"
draft: false
slug: convox_domain_tls_cert_disable
url: /configuration/rack-parameters/aws/convox_domain_tls_cert_disable
---

# convox_domain_tls_cert_disable

## Description
The `convox_domain_tls_cert_disable` parameter disables the generation of TLS certificates for the `*.convox.cloud` domain for your services. This can be useful if you are using custom domain certificates and do not require the automatic generation of Convox domain certificates.

## Default Value
The default value for `convox_domain_tls_cert_disable` is `false`.

## Use Cases
- **Custom Domain Certificates**: Use this parameter if you have internal domain certificates configured and do not need additional certificates generated for the `*.convox.cloud` domain.
- **Security and Compliance**: Managing your own certificates can give you more control over the security and compliance aspects of your deployment.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
convox_domain_tls_cert_disable  false
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `convox_domain_tls_cert_disable` parameter, use the following command:
```html
$ convox rack params set convox_domain_tls_cert_disable=true -r rackName
Setting parameters... OK
```
This command disables the automatic generation of TLS certificates for the `*.convox.cloud` domain.

## Additional Information
Disabling the automatic generation of Convox domain certificates is beneficial if you are managing your own certificates or using custom domains. It is crucial not to disable this feature unless you have a valid custom domain certificate in place to ensure secure communication for your services. For more information on managing TLS certificates and setting up custom domains with Convox, refer to the [Convox documentation on custom domains](https://docs.convox.com/deployment/custom-domains/).
