---
title: "router_log_redact_params"
description: "The router_log_redact_params AWS rack parameter masks query parameter values in router access and error logs, unset by default."
slug: router_log_redact_params
url: /configuration/rack-parameters/aws/router_log_redact_params
---

# router_log_redact_params

## Description

The `router_log_redact_params` parameter masks query parameter values in the access and error lines of the Rack's public `ingress-nginx` router, and of the internal router when [`internal_router`](/configuration/rack-parameters/aws/internal_router) is on. Fluentd applies the mask before it writes the lines to the Rack system log group `/convox/<rack>/system` in CloudWatch, which `convox rack logs` reads, and before it forwards them to [`syslog`](/configuration/rack-parameters/aws/syslog). A masked value reads `FILTERED`, and the parameter name is kept.

| Value | What is masked |
|:------|:---------------|
| unset (default) | nothing |
| `all` | every query parameter value |
| a comma-separated list of names, such as `reset_password_token,token,code,api_key` | the values of those parameters only |

The public router's access line is JSON, with the query string in the `request_query` field and the referer in `http_referrer`. Both fields are masked. The internal router's access lines and every error line carry the request line and referer as text, and those are masked the same way.

| Value | Before | After |
|:------|:-------|:------|
| `token,api_key,code` | `"request_query": "token=S1&page=2&API_KEY=S2"` | `"request_query": "token=FILTERED&page=2&API_KEY=FILTERED"` |
| `token,api_key,code` | `"http_referrer": "https://example.com/cb?code=S3&token=S4&keep=yes"` | `"http_referrer": "https://example.com/cb?code=FILTERED&token=FILTERED&keep=yes"` |
| `token,api_key,code` | `request: "POST /upload?token=S5&page=3 HTTP/2.0"` in an error line | `request: "POST /upload?token=FILTERED&page=3 HTTP/2.0"` |
| `all` | `"request_query": "token=S1&page=2&API_KEY=S2&flag&empty="` | `"request_query": "token=FILTERED&page=FILTERED&API_KEY=FILTERED&flag&empty=FILTERED"` |

## Default Value

The parameter is unset by default, and nothing is masked. `convox rack params` lists the parameter only once it has been set.

## Accepted Values

- `all`, on its own and in lowercase.
- A comma-separated list of parameter names with no spaces. Each name uses letters, digits, `_`, `.` and `-`.

Names match in any case, so `api_key` also masks `API_KEY` and `Api_Key`. An `_` in a name also matches its percent-encoded form `%5F`.

## Use Cases

- **Credentials in URLs**: mask password reset tokens, OAuth authorization codes or API keys that clients send in query strings, so they do not reach CloudWatch or your syslog receiver.
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
- Router logs on a Rack with [`router_type=contour`](/configuration/rack-parameters/aws/router_type). Convox does not collect Envoy's logs, so the parameter has no effect there.

## Additional Information

- **Validation:** the CLI rejects `ALL`, or `all` combined with names, with `param 'router_log_redact_params' takes 'all' on its own and in lowercase, or a comma-separated list of names`. It rejects a name with any other character, such as a space or `[`, with `param 'router_log_redact_params' has an invalid name "<name>": use letters, digits, '_', '.' and '-', separated by commas`.
- **Version:** requires Rack version `3.25.11` or later and CLI version `3.25.11` or later. An older CLI rejects the parameter with `unknown parameter 'router_log_redact_params' for aws provider`, and cannot clear it.
- **Downgrade:** downgrading below `3.25.11` removes the parameter with `NOTICE: removing parameters not supported by version <version>: router_log_redact_params` on stderr, and router lines are stored and forwarded unmasked from then on. A Rack managed through the Console keeps the stored value and applies it again on the next update to `3.25.11` or later. A self-managed Rack drops the value, so set it again after upgrading.
- **Providers:** AWS, GCP, Azure, DigitalOcean and metal. See [GCP](/configuration/rack-parameters/gcp/router_log_redact_params), [Azure](/configuration/rack-parameters/azure/router_log_redact_params) and [DigitalOcean](/configuration/rack-parameters/do/router_log_redact_params). On metal Racks, set it through the CLI.

## See Also

- [syslog](/configuration/rack-parameters/aws/syslog) for the endpoint and what Fluentd forwards
- [syslog_access_logs](/configuration/rack-parameters/aws/syslog_access_logs) for forwarding the public router's access logs to `syslog`
- [Ingress Router](/configuration/ingress-router#router-logs) for where router logs go
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
