---
title: "convox_domain_tls_cert_disable"
draft: false
slug: convox_domain_tls_cert_disable
url: /configuration/rack-parameters/aws/convox_domain_tls_cert_disable
---

# convox_domain_tls_cert_disable

## Description
The `convox_domain_tls_cert_disable` parameter allows you to disable the automatic generation of TLS certificates for the `*.convox.cloud` domain for your services. This can be useful when you are exclusively using custom domains and want to reduce the number of TLS certificates being generated and managed.

When enabled (set to `true`), Convox will not request or provision certificates for the default `convox.cloud` domains, which can help to stay within certificate quota limits and reduce certificate management overhead.

## Default Value
The default value for `convox_domain_tls_cert_disable` is `false`, meaning Convox will automatically generate TLS certificates for default `convox.cloud` domains.

## Use Cases
- **Certificate Quota Management**: Avoid reaching Let's Encrypt's rate limits when you have many services and are only using custom domains.
- **Resource Optimization**: Reduce resource usage related to certificate management when the default domains are not needed.
- **Custom Domain Environments**: Streamline certificate management in environments where all services use custom domains exclusively.

## Setting Parameters
To disable TLS certificate generation for `convox.cloud` domains, use the following command:
```html
$ convox rack params set convox_domain_tls_cert_disable=true -r rackName
Setting parameters... OK
```

To re-enable certificate generation (if needed later), use:
```html
$ convox rack params set convox_domain_tls_cert_disable=false -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter only affects the default `*.convox.cloud` domains. Custom domains specified in your `convox.yml` will still have certificates generated.
- For existing applications, you need to redeploy your app after changing this parameter to apply the new certificate configuration.
- To verify certificate generation status for an application, you can run:
  ```bash
  # Configure kubectl to point to your Convox rack
  $ convox rack kubeconfig -r rackName > ~/.kube/config
  
  # List certificates in your application namespace
  $ kubectl get certificate -n rackName-appName
  ```
- Disabling certificate generation for `convox.cloud` domains does not affect the functionality of your applications—they will still be accessible through their default URLs, but browsers will show security warnings due to the missing certificates.
- Consider using this parameter in conjunction with [custom domains](/deployment/custom-domains) for your services to maintain secure HTTPS connections.

## Version Requirements
This feature is available in all recent versions of Convox.
