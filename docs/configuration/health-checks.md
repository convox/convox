---
title: "Health Checks"
description: "Health checks tell the load balancer whether a Process is ready to serve requests, with options for path, port, interval, grace period, and timeout."
slug: health-checks
url: /configuration/health-checks
---
# Health Checks

Deploying a [Service](/reference/primitives/app/service) behind a load balancer requires a health check to determine whether a given [Process](/reference/primitives/app/process) is ready to handle requests.

Health checks must return a valid HTTP response code (200-399) within the configured `timeout`.

[Processes](/reference/primitives/app/process) that fail three consecutive health checks are assumed dead and will be terminated and replaced.

## Defining Health Checks

### Simple Health Check
```yaml
services:
  web:
    health: /check
```
> Specifying `health` as a string will set the `path` and leave the other options as defaults.

### Advanced Health Check

```yaml
services:
  web:
    health:
      grace: 5
      interval: 5
      path: /check
      port: 8080
      timeout: 3
```

| Attribute  | Default | Description                                                                      |
| ---------- | ------- | -------------------------------------------------------------------------------- |
| **grace**    | `interval` | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks. Defaults to the value of `interval` |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | The HTTP endpoint that will be requested                                         |
| **port**     | Main service port | The port the readiness probe connects to. Set when the health endpoint listens on a different port than the main service port. Accepts a scalar (`port: 8080`) or a map with `port` and `scheme` |
| **timeout**  | `interval - 1` | The number of seconds to wait for a valid response. Defaults to `interval` minus one |
| **disable**  | false   | Set to `true` to disable the readiness probe. A liveness probe configured with `liveness.path` is unaffected |

### Separate Health Port

When your health endpoint runs on a different port than the main service port, set `health.port` so the readiness probe targets the right port without changing the load-balanced service port.

```yaml
services:
  web:
    port: https:8080
    health:
      path: /healthz
      port:
        port: 9090
        scheme: http
```

With this configuration, traffic flows to the service on HTTPS port 8080, while the readiness probe queries `http://<pod>:9090/healthz`, a plain-HTTP diagnostics endpoint fronted by an HTTPS service port.

If `scheme` is omitted, the readiness probe inherits the scheme of the main service port. The scalar form `port: 9090` is equivalent to the map form `port: { port: 9090 }` and inherits the service scheme. Provide `scheme` explicitly only when you need to override the inherited value.

> **Scope.** `health.port` only affects the readiness probe. Startup probes continue to target the main service port regardless of `health.port`, and liveness probes use their own `liveness.port` override (see below). If your service speaks gRPC (`port: grpc:...` with `grpcHealthEnabled: true`), a `health.port` override redirects the gRPC readiness probe to the new port; `scheme` is ignored in that case, and the gRPC liveness probe still follows `liveness.port`.

> **Valid scheme values.** Only `http` and `https` are valid under `health.port.scheme` and `liveness.port.scheme`. Setting `scheme: grpc` on a service whose main port is **not** gRPC will silently skip the probe. With `grpcHealthEnabled: true`, `health.port.scheme: grpc` instead activates the gRPC probes, putting readiness on `health.port` and liveness on `liveness.port`; `liveness.port.scheme: grpc` only drops the liveness probe. Use `grpcHealthEnabled: true` with a gRPC main port instead. See [gRPC Health Checks](#grpc-health-checks) below.

## Liveness Checks

Liveness checks complement health checks by monitoring the ongoing health of running processes. While health checks (readiness probes) determine when a service is ready to receive traffic, liveness checks determine when a service should be restarted if it becomes unresponsive or enters a broken state.

When a liveness check fails, Kubernetes will restart the container, which can help recover from deadlocks, memory leaks, or other issues that cause a process to become unresponsive while still appearing to be running.

### Liveness Check Configuration

```yaml
services:
  web:
    liveness:
      path: /liveness/check
      port: 9090
      grace: 15
      interval: 5
      timeout: 3
      successThreshold: 1
      failureThreshold: 3
```

| Attribute           | Default | Description                                                                      |
| ------------------- | ------- | -------------------------------------------------------------------------------- |
| **path**              |         | **Required.** The HTTP endpoint that will be requested for liveness checks      |
| **port**              | Main service port | The port the liveness probe connects to. Set when the liveness endpoint listens on a different port than the main service port. Accepts a scalar (`port: 9090`) or a map with `port` and `scheme` |
| **grace**             | 10      | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to start before beginning liveness checks |
| **interval**          | 5       | The number of seconds between liveness checks                                    |
| **timeout**           | 5       | The number of seconds to wait for a successful response                          |
| **successThreshold**  | 1       | The number of consecutive successful checks required to consider the probe successful |
| **failureThreshold**  | 3       | The number of consecutive failed checks required before restarting the container |

### Important Considerations

- **Path is Required**: Unlike health checks, an HTTP service must specify a `path` to enable liveness checks. Setting `liveness.port` without `liveness.path` is a silent no-op, so no liveness probe is generated. A gRPC service using `grpcHealthEnabled` is the exception, see [gRPC Health Checks](#grpc-health-checks)
- **`port` is optional**: If unset, the liveness probe targets the main service port, mirroring the `health.port` behavior. Startup probes ignore `liveness.port` and continue to target the main service port. If you need the liveness probe to use a specific scheme (for example, HTTPS), set `liveness.port.scheme` explicitly. Unlike readiness, liveness does not auto-inherit the main service scheme
- **Conservative Configuration**: Liveness checks should be configured conservatively to avoid unnecessary restarts. False positives can cause service disruption
- **Separate Endpoints**: Consider using different endpoints for health checks and liveness checks to monitor different aspects of your application
- **Startup Time**: Set an appropriate `grace` period to allow your application to fully initialize before liveness checks begin

### Example Use Cases

**Detecting Deadlocks:**
```yaml
services:
  worker:
    liveness:
      path: /worker/health
      grace: 30
      interval: 10
      failureThreshold: 5
```

**Monitoring Memory-Intensive Applications:**
```yaml
services:
  processor:
    liveness:
      path: /memory-check
      grace: 45
      interval: 15
      timeout: 10
      failureThreshold: 3
```

## Startup Probes

Startup probes provide a way to check if an application has successfully started before allowing readiness and liveness probes to take effect. This is particularly useful for applications that require significant initialization time or have variable startup durations.

When a startup probe is configured, all other probes are disabled until it succeeds. This prevents Kubernetes from prematurely marking a service as unhealthy or restarting it before initialization completes.

### Startup Probe Configuration

A startup probe requires either a `path` (HTTP check) or `tcpSocketPort` (TCP check) to define what is checked. Timing parameters (grace, interval, timeout, thresholds) can be set directly on the startup probe. If not explicitly set, they are inherited from the **liveness** check configuration.

> You must configure a `liveness` check alongside your startup probe. If startup probe timing fields are not explicitly set, they fall back to the liveness values. Without a liveness configuration and without explicit startup probe timing, these values default to zero, which will cause immediate failures.

#### TCP Startup Probe

```yaml
services:
  web:
    build: .
    port: 3000
    startupProbe:
      tcpSocketPort: 3000
    liveness:
      path: /live
      grace: 30
      interval: 10
      timeout: 5
      successThreshold: 1
      failureThreshold: 30
```

| Attribute           | Description                                                                      |
| ------------------- | -------------------------------------------------------------------------------- |
| **tcpSocketPort**   | **Required** (if `path` not set). The TCP port to check for startup success      |

#### HTTP Startup Probe

```yaml
services:
  api:
    build: .
    port: 8080
    startupProbe:
      path: /startup
    liveness:
      path: /live
      grace: 10
      interval: 5
      timeout: 3
      failureThreshold: 40
```

| Attribute           | Description                                                                      |
| ------------------- | -------------------------------------------------------------------------------- |
| **path**            | **Required** (if `tcpSocketPort` not set). The HTTP endpoint to check for startup success |

### Timing Configuration

The startup probe supports its own timing parameters. When explicitly set, these values are used directly. When not set (value is `0`), they fall back to the corresponding liveness check values:

| Attribute              | Description                              | Fallback (when not set) |
| ---------------------- | ---------------------------------------- | ----------------------- |
| **grace**              | Initial delay in seconds before probing  | `liveness.grace` (default: 10)    |
| **interval**           | Seconds between probe attempts           | `liveness.interval` (default: 5)  |
| **timeout**            | Seconds before probe times out           | `liveness.timeout` (default: 5)   |
| **successThreshold**   | Consecutive successes to be considered ready | `liveness.successThreshold` (default: 1) |
| **failureThreshold**   | Consecutive failures before pod is killed | `liveness.failureThreshold` (default: 3) |

Set these directly on the startup probe to use different values from the liveness check:

```yaml
services:
  api:
    startupProbe:
      path: /startup
      grace: 60
      interval: 30
      timeout: 5
      failureThreshold: 10
    liveness:
      path: /live
      grace: 10
      interval: 5
      timeout: 3
      failureThreshold: 3
```

In this example, the startup probe uses its own values: 60s grace, 30s interval, 10 failure threshold, allowing a generous startup window (60s + 10 x 30s = 360s) while keeping the liveness probe tight for ongoing monitoring.

> **Note on versions before 3.24.1:** In rack versions prior to 3.24.1, a bug caused the startup probe to always use liveness timing values regardless of explicit configuration. If you are running an older rack version, timing fields set on `startupProbe` will have no effect. Update to 3.24.1 or later to use independent startup probe timing.

### Use Cases for Startup Probes

Startup probes are ideal for:

- **Database Migrations**: Applications that run database migrations on startup
- **Cache Warming**: Services that need to populate caches before serving traffic
- **Large Applications**: Applications with significant initialization requirements
- **Configuration Loading**: Services that load extensive configuration or connect to multiple external services
- **Legacy Applications**: Applications with unpredictable or lengthy startup times

### Example: Application with Long Initialization

```yaml
services:
  analytics:
    build: .
    port: 5000
    startupProbe:
      tcpSocketPort: 5000
    liveness:
      path: /live
      grace: 60
      interval: 15
      timeout: 5
      failureThreshold: 20
    health:
      path: /health
      interval: 5
```

In this example:
- The startup probe checks TCP port 5000 using the liveness timing: every 15 seconds, up to 20 failures, allowing approximately 5 minutes for startup (15s × 20)
- Once the startup probe succeeds, the health (readiness) and liveness checks begin
- If startup fails after 20 attempts, the container is restarted

### Important Startup Probe Considerations

- **Relationship with Other Probes**: Liveness and readiness probes are disabled until the startup probe succeeds
- **Liveness Required**: You must define a liveness check alongside your startup probe. If startup probe timing fields are not explicitly set, they fall back to the liveness values
- **Independent Timing**: Timing fields (`grace`, `interval`, `timeout`, `successThreshold`, `failureThreshold`) can be set directly on the startup probe to use values independent of the liveness configuration (requires rack version 3.24.1+)
- **Failure Threshold**: Set a high enough `failureThreshold` on the startup probe (or on the liveness check if relying on fallback) to accommodate your application's maximum startup time
- **Startup vs. Liveness**: Use startup probes for initialization, liveness probes for ongoing health monitoring
- **Resource Planning**: Consider that pods may take longer to become ready when using startup probes

## gRPC Health Checks

For services that use gRPC instead of HTTP, Convox provides support for gRPC health checks through the gRPC health checking protocol. To enable gRPC health checks, you need to:

1. Specify that your service uses the gRPC protocol in the port definition
2. Enable the gRPC health check with the `grpcHealthEnabled` attribute

### Basic Configuration

```yaml
services:
  api:
    build: .
    port: grpc:50051
    grpcHealthEnabled: true
```

### Advanced Configuration

A subset of the `health` attributes applies to gRPC health checks, along with two `liveness` attributes. See the tables below.

```yaml
services:
  api:
    build: .
    port: grpc:50051
    grpcHealthEnabled: true
    health:
      grace: 20
      interval: 5
      grpcService: myapp.v1.Greeter
      timeout: 2
    liveness:
      grpcService: myapp.v1.Liveness
```

| Attribute  | Default | Description                                                                      |
| ---------- | ------- | -------------------------------------------------------------------------------- |
| **grace**    | `interval` | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks. Defaults to the value of `interval` |
| **grpcService** | empty | The gRPC health service name placed in the `HealthCheckRequest`. Empty means overall server health. Affects the readiness probe only. Requires rack version 3.25.4 or later |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | Has no effect on a gRPC service. Use `grpcService`                               |
| **port**     | Main service port | Moves the gRPC readiness probe to another port. `scheme` is ignored. The liveness probe follows `liveness.port` |
| **timeout**  | `interval - 1` | The number of seconds to wait for a valid response. Defaults to `interval` minus one |
| **disable**  | false   | Set to `true` to disable the gRPC readiness and liveness probes. Requires rack version 3.25.4 or later |

These two `liveness` attributes apply as well.

| Attribute  | Default | Description                                                                      |
| ---------- | ------- | -------------------------------------------------------------------------------- |
| **grpcService** | empty | The gRPC health service name placed in the liveness probe's `HealthCheckRequest`. Empty means overall server health. Requires rack version 3.25.5 or later. An earlier rack accepts the key without error and ignores it, leaving the liveness probe on the empty name |
| **port**     | Main service port | Moves the gRPC liveness probe to another port |

No other `liveness` attribute reaches the gRPC probes. The `liveness.*` timing fields are still the fallback timings for a [startup probe](#startup-probes).

`grpcService` takes a gRPC service name such as `myapp.v1.Greeter`. It is not a URL path: a value beginning with `/` is rejected when your app is built, so moving an old `path: /` value into this key fails fast rather than at rollout. Beyond that the value is passed through as written, and the name must be registered on your server. `Check` returns `NOT_FOUND` for a name your server does not know. In `health.grpcService` that means readiness never passes and the deploy does not converge; in `liveness.grpcService` it means the liveness probe fails and the kubelet restarts the container.

`health.grpcService` changes the readiness probe only. The liveness probe sends the empty service name unless you also set `liveness.grpcService`, so by default your server must report `SERVING` for both the empty name and the name you set. Go's `health.NewServer()` registers the empty name for you. A hand-written `Check` method, or a health registry you populate yourself, must handle an empty `req.Service` as well, or the liveness probe fails and the container restarts. Registering the empty name is one line and is safe to drain against, so prefer it whenever you control the server.

`liveness.grpcService` is for the cases where you cannot: a server you do not build, or one that deliberately exposes a separate liveness service. Point it at a name you never flip to `NOT_SERVING`. Setting it to the same name as `health.grpcService` means a drain fails liveness as well, and the container is restarted after five failed checks instead of being taken out of rotation, which is the opposite of what a drain is for. Your build prints a warning to the deploy output when the two names match.

If `liveness.port` points at a separate health listener, only that listener has to answer the name the liveness probe sends.

Before rack version 3.25.4, `health.disable: true` was ignored on a gRPC service and both probes rendered anyway. Updating a rack to 3.25.4 with `health.disable: true` already set on a gRPC service removes both of its probes.

### Separate Liveness Listener

To keep the two probes fully independent, give liveness its own port:

```yaml
services:
  api:
    build: .
    port: grpc:50051
    grpcHealthEnabled: true
    health:
      grpcService: myapp.v1.Greeter
    liveness:
      port: 50052
```

Readiness now checks `myapp.v1.Greeter` on port 50051 and liveness checks the empty name on port 50052. Run a second health listener on 50052 that always reports `SERVING` for the empty name, and the main registry only has to answer for the name you set. Flipping `myapp.v1.Greeter` to `NOT_SERVING` at runtime then takes the pod out of rotation without restarting the container, which is the drain behavior `grpcService` exists for.

Unlike an HTTP service, a gRPC service needs no `liveness.path` for this: the liveness probe is generated by `grpcHealthEnabled` and `liveness.port` moves it.

From rack version 3.25.5 you can separate the two checks on a single port by giving each one its own name, with `health.grpcService` for readiness and `liveness.grpcService` for liveness. A second listener is still the better answer when the checks need to be independent of the main port itself, for example when that port can saturate.

### Implementation Requirements

Services using gRPC health checks must implement the gRPC Health Checking Protocol, which is defined in the [gRPC health checking protocol repository](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).

This protocol requires your service to implement a `Health` service with a `Check` method that returns the service's health status.

### Probe Behavior

When `grpcHealthEnabled` is set to `true`, Convox configures both:

1. A **readinessProbe** - Determines whether the service is ready to receive traffic
2. A **livenessProbe** - Determines whether the service should be restarted

The readinessProbe ensures that gRPC services won't receive traffic until they are fully ready, while the livenessProbe monitors the ongoing health of the service and initiates restarts if necessary.

Both probes use `health.grace`, `health.interval` and `health.timeout`. They do not share a port: the readiness port comes from `health.port` and the liveness port from `liveness.port`, each defaulting to the main service port.

`grpcHealthEnabled: true` is required. A `port: grpc:NNNN` service without it gets no readiness probe and no liveness probe at all.

`grpcHealthEnabled: true` is inert on a service whose main port is not gRPC, unless you also set `health.port.scheme: grpc`. That activates the gRPC probes, giving you a gRPC readiness probe on `health.port` and a gRPC liveness probe on `liveness.port`, which defaults to the main port. If `liveness.path` is also set, that HTTP liveness probe is replaced by the gRPC one. Set `liveness.port` to match, or leave `health.port.scheme` unset.

For the gRPC probes, only `liveness.port` and `liveness.grpcService` have any effect; the `liveness.*` timing fields and `liveness.path` do not. Those timings are still used as the fallback timings for a [startup probe](#startup-probes), including the automatic startup probe added to GPU services.

`startupProbe.path` renders a plain HTTP GET against the gRPC port and fails unless your server also answers HTTP/1.1 there. Use `startupProbe.tcpSocketPort` on a gRPC service.

> gRPC probes use a hardcoded `failureThreshold` of **5** and `successThreshold` of **1** for both readiness and liveness. This differs from HTTP probes, where readiness uses a `failureThreshold` of **3** and liveness uses the configurable `liveness.failureThreshold` (default **3**). The gRPC thresholds are not configurable.

### Example Implementation

Here's a minimal example of a gRPC health check implementation in Go:

```go
import (
	"context"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	server := grpc.NewServer()
	
	// Register your service
	// pb.RegisterYourServiceServer(server, &yourServiceImpl{})
	
	// Register the health service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	
	// The liveness probe checks the empty service name unless you set liveness.grpcService
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	
	// Only needed when you set health.grpcService, which the readiness probe checks.
	// Flip this to NOT_SERVING to drain traffic without restarting the container.
	healthServer.SetServingStatus("myapp.v1.Greeter", healthpb.HealthCheckResponse_SERVING)
	
	// Only needed when you set liveness.grpcService. Never flip this one to NOT_SERVING.
	healthServer.SetServingStatus("myapp.v1.Liveness", healthpb.HealthCheckResponse_SERVING)

	// Continue with server initialization...
}
```

With this implementation and the appropriate configuration in your `convox.yml`, your gRPC service will properly report its health status to Convox, ensuring that it only receives traffic when it's ready to handle requests.

## See Also

- [Service Lifecycle Hooks](/reference/primitives/app/service#lifecycle) for preStop and postStart container hooks
- [Load Balancers](/configuration/load-balancers) for configuring traffic routing
- [Rolling Updates](/deployment/rolling-updates) for how health checks affect deployments
- [Scaling](/configuration/scaling) for autoscaling configuration
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing health check failures during deployment
