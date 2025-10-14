---
title: "SSL"
draft: false
slug: SSL
url: /deployment/ssl
---
# SSL

Convox will automatically generate a valid SSL certificate for your service using Let's Encrypt. By default, Convox handles SSL certificate generation via HTTP-01 and email validation methods.

If you specify a custom `domain:` attribute for your service, Convox will automatically generate and validate an SSL certificate using Let's Encrypt's default HTTP-01 challenge. You may also receive an email for domain ownership verification during this process.

## Pre-generate your certificate

To minimize delays during your first deployment, you can pre-generate your SSL certificate. This ensures your service is available without waiting for the certificate issuance process.

```html
$ convox certs generate "*.example.org" "myapp.example.org" --issuer letsencrypt
Generating certificate... OK, cert-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

This command will initiate the HTTP-01 challenge or email verification for domain ownership, allowing the certificate to be issued and ready for use during deployment.

## Certificate Management

You can view and manage your existing SSL certificates with the following commands:

To list your current certificates:

```html
$ convox certs --generated
ID                          DOMAIN                   EXPIRES
cert-xxxxxxxxxxxxxx         *.example.com            1 year from now
cert-yyyyyyyyyyyyyy         myapp.example.org        1 year from now
```

To delete an existing certificate:

```html
$ convox certs delete cert-xxxxxxxxxxxxxx
Deleting certificate cert-xxxxxxxxxxxxxx... OK
```

## Advanced SSL Configuration: Let's Encrypt DNS01 Challenge

Convox supports the Let's Encrypt DNS01 challenge for SSL certificate generation, which is useful when HTTP endpoints are not exposed or when wildcard certificates are required. The DNS01 challenge verifies domain ownership via DNS TXT records, and it is ideal for environments with strict security requirements.

### Using AWS Route53

### Setting Up DNS01 Challenge

1. **Retrieve the IAM Role**: Retrieve the IAM role used by the service:

```html
convox letsencrypt dns route53 role
```

This will return the ARN of the IAM role (e.g., `arn:aws:iam::XXXXXXXXXX:role/convox/rackName-cert-manager`).

2. **Create a Route53 DNS Zone Access Role**: Create a role in AWS IAM with the necessary permissions to manage your Route53 DNS zone. Replace `<zone-id>` with your actual Route53 zone ID:

```html
{
   "Version": "2012-10-17",
   "Statement": [
       {
           "Effect": "Allow",
           "Action": "route53:GetChange",
           "Resource": "arn:aws:route53:::change/*"
       },
       {
           "Effect": "Allow",
           "Action": [
               "route53:ChangeResourceRecordSets",
               "route53:ListResourceRecordSets"
           ],
           "Resource": "arn:aws:route53:::hostedzone/<zone-id>"
       }
   ]
}
```

3. **Update Trust Policy**: Update the trust policy for this role to allow the `cert-manager` role from your Convox rack to assume it:

```html
{
   "Version": "2012-10-17",
   "Statement": [
       {
           "Effect": "Allow",
           "Principal": {
               "AWS": [
                   "arn:aws:iam::XXXXXXXXXX:role/convox/rackName-cert-manager"
               ]
           },
           "Action": "sts:AssumeRole"
       }
   ]
}
```

4. **Add Assume Permission**: Add assume role permission to your `cert-manager` role:

```html
{
   "Version": "2012-10-17",
   "Statement": [
       {
           "Effect": "Allow",
           "Action": "sts:AssumeRole",
           "Resource": [
               "arn:aws:iam::XXXXXXXXXX:role/dns-access"
           ]
       }
   ]
}
```

5. **Configure the DNS Solver**: Run the following command to configure the DNS solver for Let's Encrypt:

```html
convox letsencrypt dns route53 add --id 1 --dns-zones <your.zone> --role arn:aws:iam::XXXXXXXXXX:role/dns-access --hosted-zone-id <hosted-zone-id> --region <hosted-zone-region>
```

6. **Verify Configuration**: Check the configuration:

```html
convox letsencrypt dns route53 list
```

This command will list your DNS zones and hosted zone IDs, confirming that the DNS01 challenge is configured correctly.

### Using Cloudflare

If you manage DNS in Cloudflare, you can hand the token or API key value directly to Convox and it will manage the underlying secret for you.

1. **Configure the DNS Solver**: Run the following command to register the Cloudflare solver with Convox. Supplying `--api-token` causes Convox to store the token in a secret named `cloudflare-dns-credential-<id>` under the `cert-manager` namespace automatically.

```html
convox letsencrypt dns cloudflare add --id 1 --dns-zones <your.zone> \
  --api-token <your_api_token>
```

   To use an API key instead, swap `--api-token` for `--api-key <your_api_key>` and add `--email <cloudflare_account_email>`. Convox stores both the key and email alongside one another in the same secret so cert-manager can authenticate correctly.

2. **Verify Configuration**: Check the configuration:

```html
convox letsencrypt dns cloudflare list
```

This command will list your Cloudflare-backed DNS zones and the referenced Kubernetes secrets, confirming that the DNS01 challenge is configured correctly.

## Wildcard Certificates and Reuse

Convox allows you to generate wildcard certificates that secure an entire domain and all its subdomains (e.g., `*.example.com`). Additionally, wildcard certificates can be reused across multiple apps and services, making SSL management more efficient.

### Generating Wildcard Certificates

To generate a wildcard certificate:

```html
$ convox certs generate "*.mydomain.com" --issuer letsencrypt
```

### Reusing Wildcard Certificates

Once the wildcard certificate is generated, you can reuse it across multiple apps by referencing the certificate ID in your `convox.yml` file.

For example, to apply the wildcard certificate to a service:

```html
environment:
  - PORT=3000
services:
  web:
    build: .
    domain: my-app.mydomain.com
    port: 3000
    certificate:
      id: cert-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

This allows you to securely use the same SSL certificate across multiple apps or services, reducing the administrative overhead of managing multiple certificates.

## Conclusion

Convox simplifies SSL certificate management by integrating Let's Encrypt for both standard HTTP-01 validation and more advanced DNS01 challenges. Whether you need to secure single domains, multiple subdomains, or reuse wildcard certificates across apps, Convox provides flexible options for securing your services.
