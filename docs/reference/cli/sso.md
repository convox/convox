---
title: "sso"
draft: false
slug: sso   
url: /reference/cli/sso
---
# SSO

## configure

Configure necessary fields to make the authentication using `login` command.
Fields: 
- Provider: Which identity provider will be used to authenticate. Supported providers: `okta`.
- Client ID: The OpenID Connect client ID provided by your Identity Provider.
- Client secret: The OpenID Connect client secret provided by your Identity Provider.
- Issuer: A unique string that identifies the provider issuing a request.

### Usage
```html
    convox sso configure
```

or

```html
    convox sso configure -p PROVIDER -c CLIENT_ID -s CLIENT_SECRET -i ISSUER
```

or we can add following environment variables:

```
SSO_PROVIDER
SSO_CLIENT_ID
SSO_CLIENT_SECRET
SSO_ISSUER
```

### Examples
```html
    convox sso configure
    SSO Provider: 
    SSO Client ID: 
    SSO Client Secret: 
    SSO Issuer: 
    OK
```

## login

Authenticate your CLI using a identity provider.

### Usage
```html
    convox sso login
```

You will be redirected to the browser to your fill the credentials in your identity provider.

### Requirements
Is necessary to configure the identity provider to add the `convoxID` that refers with the user id in the console. Also, is required to add this field in the token claim. 
Also, configure the callback endpoint in the customer application as `http://localhost:8090/authorization-code/callback`.