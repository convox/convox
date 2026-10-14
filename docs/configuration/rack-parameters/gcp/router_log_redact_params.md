---
title: "router_log_redact_params"
description: "The router_log_redact_params GCP rack parameter masks query parameter values in router access and error logs, unset by default."
slug: router_log_redact_params
url: /configuration/rack-parameters/gcp/router_log_redact_params
---

# router_log_redact_params

## Description

The `router_log_redact_params` parameter masks query parameter values in the access and error lines of the Rack's `ingress-nginx` router. Fluentd applies the mask before it writes the lines to the Rack's Elasticsearch, which `convox rack logs` reads, and before it forwards them to [`syslog`](/configuration/rack-parameters/gcp/syslog). A masked value reads `FILTERED`, and the parameter name is kept.

| Value | What is masked |
|:------|:---------------|
| unset (default) | nothing |
| `all` | every query parameter value |
| a comma-separated list of names, such as `reset_password_token,token,code,api_key` | the values of those parameters only |

The router's access lines carry the request line, with its query string, and the referer as quoted text. Both are masked, and so is the request line in the router's error lines.

| Value | Before | After |
|:------|:-------|:------|
| `token,api_key,code` | `"GET /search?token=S1&page=2&API_KEY=S2 HTTP/2.0"` | `"GET /search?token=FILTERED&page=2&API_KEY=FILTERED HTTP/2.0"` |
| `token,api_key,code` | `"https://example.com/cb?code=S3&token=S4&keep=yes"` as the referer | `"https://example.com/cb?code=FILTERED&token=FILTERED&keep=yes"` |
| `token,api_key,code` | `request: "POST /upload?token=S5&page=3 HTTP/2.0"` in an error line | `request: "POST /upload?token=FILTERED&page=3 HTTP/2.0"` |
| `all` | `"GET /search?token=S1&page=2&API_KEY=S2&flag&empty= HTTP/2.0"` | `"GET /search?token=FILTERED&page=FILTERED&API_KEY=FILTERED&flag&empty=FILTERED HTTP/2.0"` |

## Default Value

The parameter is unset by default, and nothing is masked. `convox rack params` lists the parameter only once it has been set.

## Accepted Values

- `all`, on its own and in lowercase.
- A comma-separated list of parameter names with no spaces. Each name uses letters, digits, `_`, `.` and `-`.

Names match in any case, so `api_key` also masks `API_KEY` and `Api_Key`. An `_` in a name also matches its percent-encoded form `%5F`.

## Use Cases

- **Credentials in URLs**: mask password reset tokens, OAuth authorization codes or API keys that clients send in query strings, so they do not reach Elasticsearch or your syslog receiver.
- **Unknown parameter names**: set `all` when you cannot list every sensitive name in advance.

## Setting Parameters

```bash
$ convox rack params set router_log_redact_params=reset_password_token,token,code,api_key -r rackName
Updating parameters... OK
```

To mask every query parameter value:

```bash
$ convox rack params set router_log_redact_params=all -r rackName
Updating parameters... OK
```

To stop masking, clear the value:

```bash
$ convox rack params set router_log_redact_params= -r rackName
Updating parameters... OK
```

Setting or clearing the parameter rolls the Fluentd pods on every node. The new value applies to lines written after the roll. Lines already stored keep the form they were stored in.

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## What It Does Not Mask

- Path segments, such as the token in `/reset/<token>`.
- Bracketed names such as `user[token]` or `user%5Btoken%5D`, and names with percent-encoded characters other than `_`, unless the value is `all`.
- Your Apps' own log lines. The parameter applies to router lines only.
- The router's container logs on the node, which `kubectl logs` reads. Fluentd masks the copy it stores and forwards.
- The copy of the router's output that GKE sends to Cloud Logging. Convox leaves the cluster's Cloud Logging settings as GKE sets them.

## Additional Information

- **Validation:** the CLI rejects `ALL`, or `all` combined with names, with `param 'router_log_redact_params' takes 'all' on its own and in lowercase, or a comma-separated list of names`. It rejects a name with any other character, such as a space or `[`, with `param 'router_log_redact_params' has an invalid name "<name>": use letters, digits, '_', '.' and '-', separated by commas`.
- **Version:** requires Rack version `3.25.11` or later and CLI version `3.25.11` or later. An older CLI rejects the parameter with `unknown parameter 'router_log_redact_params' for gcp provider`, and cannot clear it.
- **Downgrade:** downgrading below `3.25.11` removes the parameter with `NOTICE: removing parameters not supported by version <version>: router_log_redact_params` on stderr, and router lines are stored and forwarded unmasked from then on. A Rack managed through the Console keeps the stored value and applies it again on the next update to `3.25.11` or later. A self-managed Rack drops the value, so set it again after upgrading.
- **Providers:** AWS, GCP, Azure, DigitalOcean and metal. See [AWS](/configuration/rack-parameters/aws/router_log_redact_params), [Azure](/configuration/rack-parameters/azure/router_log_redact_params) and [DigitalOcean](/configuration/rack-parameters/do/router_log_redact_params). On metal Racks, set it through the CLI.

## See Also

- [syslog](/configuration/rack-parameters/gcp/syslog) for the endpoint and what Fluentd forwards
- [syslog_tls_verify](/configuration/rack-parameters/gcp/syslog_tls_verify) for certificate verification on `tcp+tls` endpoints
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
