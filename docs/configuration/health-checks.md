---
title: "Health Checks"
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
      timeout: 3
```

| Attribute  | Default | Description                                                                      |
| ---------- | ------- | -------------------------------------------------------------------------------- |
| **grace**    | `interval` | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks. Defaults to the value of `interval` |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | The HTTP endpoint that will be requested                                         |
| **timeout**  | `interval - 1` | The number of seconds to wait for a valid response. Defaults to `interval` minus one |
| **disable**  | false   | Set to `true` to disable the health check entirely                               |

## Liveness Checks

Liveness checks complement health checks by monitoring the ongoing health of running processes. While health checks (readiness probes) determine when a service is ready to receive traffic, liveness checks determine when a service should be restarted if it becomes unresponsive or enters a broken state.

When a liveness check fails, Kubernetes will restart the container, which can help recover from deadlocks, memory leaks, or other issues that cause a process to become unresponsive while still appearing to be running.

### Liveness Check Configuration

```yaml
services:
  web:
    liveness:
      path: /liveness/check
      grace: 15
      interval: 5
      timeout: 3
      successThreshold: 1
      failureThreshold: 3
```

| Attribute           | Default | Description                                                                      |
| ------------------- | ------- | -------------------------------------------------------------------------------- |
| **path**              |         | **Required.** The HTTP endpoint that will be requested for liveness checks      |
| **grace**             | 10      | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to start before beginning liveness checks |
| **interval**          | 5       | The number of seconds between liveness checks                                    |
| **timeout**           | 5       | The number of seconds to wait for a successful response                          |
| **successThreshold**  | 1       | The number of consecutive successful checks required to consider the probe successful |
| **failureThreshold**  | 3       | The number of consecutive failed checks required before restarting the container |

### Important Considerations

- **Path is Required**: Unlike health checks, you must specify a `path` to enable liveness checks
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

A startup probe requires either a `path` (HTTP check) or `tcpSocketPort` (TCP check) to define what is checked. All timing parameters (grace, interval, timeout, thresholds) are inherited from the **liveness** check configuration and cannot be set independently on the startup probe.

> You must configure a `liveness` check alongside your startup probe. The startup probe uses the liveness timing values for its grace period, interval, timeout, and thresholds. Without a liveness configuration, these values default to zero, which will cause immediate failures.

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

The startup probe supports its own timing parameters. If not explicitly set, values are inherited from the liveness check:

| Attribute              | Description                              | Default (inherited from liveness) |
| ---------------------- | ---------------------------------------- | --------------------------------- |
| **grace**              | Initial delay in seconds before probing  | `liveness.grace` (default: 10)    |
| **interval**           | Seconds between probe attempts           | `liveness.interval` (default: 5)  |
| **timeout**            | Seconds before probe times out           | `liveness.timeout` (default: 5)   |
| **successThreshold**   | Consecutive successes to be considered ready | `liveness.successThreshold` (default: 1) |
| **failureThreshold**   | Consecutive failures before pod is killed | `liveness.failureThreshold` (default: 3) |

You can set these directly on the startup probe to use different values from the liveness check:

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

This allows a generous startup window (60s delay + 10 × 30s = 360s) while keeping the liveness probe tight for ongoing health monitoring.

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
- **Liveness Required**: Startup probe timing is always inherited from the liveness configuration. You must define a liveness check with appropriate timing for your startup requirements
- **Timing Fields Ignored**: Setting timing fields (`grace`, `interval`, `timeout`, `successThreshold`, `failureThreshold`) directly on the startup probe has no effect. These values are always read from the liveness configuration, even if explicitly specified under `startupProbe:`
- **Failure Threshold**: Set a high enough `failureThreshold` on the **liveness** check to accommodate your application's maximum startup time
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

You can customize the gRPC health check behavior using the same `health` attributes as HTTP health checks:

```yaml
services:
  api:
    build: .
    port: grpc:50051
    grpcHealthEnabled: true
    health:
      grace: 20
      interval: 5
      path: /
      timeout: 2
```

| Attribute  | Default | Description                                                                      |
| ---------- | ------- | -------------------------------------------------------------------------------- |
| **grace**    | `interval` | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks. Defaults to the value of `interval` |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | The service name to check within your gRPC health implementation                 |
| **timeout**  | `interval - 1` | The number of seconds to wait for a valid response. Defaults to `interval` minus one |
| **disable**  | false   | Set to `true` to disable the health check entirely                               |

### Implementation Requirements

Services using gRPC health checks must implement the gRPC Health Checking Protocol, which is defined in the [gRPC health checking protocol repository](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).

This protocol requires your service to implement a `Health` service with a `Check` method that returns the service's health status.

### Probe Behavior

When `grpcHealthEnabled` is set to `true`, Convox configures both:

1. A **readinessProbe** - Determines whether the service is ready to receive traffic
2. A **livenessProbe** - Determines whether the service should be restarted

The readinessProbe ensures that gRPC services won't receive traffic until they are fully ready, while the livenessProbe monitors the ongoing health of the service and initiates restarts if necessary.

Both probes use the health settings defined in your `convox.yml`, ensuring consistent behavior throughout the service lifecycle.

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
	
	// Set your service as serving
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	
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
