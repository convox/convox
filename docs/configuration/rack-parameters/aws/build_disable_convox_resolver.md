---
title: "build_disable_convox_resolver"
draft: false
slug: build_disable_convox_resolver
url: /configuration/rack-parameters/aws/build_disable_convox_resolver
---

# build_disable_convox_resolver

## Description
The `build_disable_convox_resolver` parameter allows you to disable the Convox DNS resolver during build processes. This can help resolve DNS resolution issues that may occur in builds with package managers like Yarn, which can sometimes struggle with multi-level DNS requests to repositories.

By setting this parameter to `true`, the Convox DNS resolver will be bypassed during builds, allowing the standard container DNS resolution to handle package manager requests.

## Default Value
The default value for `build_disable_convox_resolver` is `false`.

## Use Cases
- **Package Manager Issues**: Resolve DNS-related issues with package managers like Yarn that make multi-level DNS requests.
- **Build Failures**: Troubleshoot and fix builds that fail due to DNS resolution problems.
- **Repository Access**: Improve reliability when accessing package repositories that require complex DNS resolution.
- **Network Dependencies**: Enhance connectivity to external resources during the build process.

## Setting Parameters
To disable the Convox DNS resolver during builds, use the following command:
```html
$ convox rack params set build_disable_convox_resolver=true -r rackName
Setting parameters... OK
```

To re-enable the Convox DNS resolver (if needed):
```html
$ convox rack params set build_disable_convox_resolver=false -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter specifically affects the DNS resolution process during build operations and does not impact runtime DNS resolution for deployed applications.
- When disabled, builds will use the standard Kubernetes cluster DNS resolution instead of the Convox custom resolver.
- Common symptoms that might indicate DNS resolution issues during builds include:
  - Timeouts when fetching packages
  - Intermittent build failures
  - Package managers reporting network connectivity issues
  - Inconsistent ability to resolve external dependencies
- This parameter is particularly useful when:
  - Using Yarn or other package managers that make complex DNS requests
  - Building applications with many external dependencies
  - Experiencing intermittent build failures related to network connectivity
- After changing this parameter, you should perform a build to verify that DNS resolution issues have been resolved.
- If disabling the Convox DNS resolver does not resolve your build issues, other network-related factors may be involved.

## Version Requirements
This feature requires at least Convox rack version `3.18.3`.
