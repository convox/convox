---
title: "Health Checks"
draft: false
slug: Health Checks
url: /configuration/health-checks
---
# Health Checks

Deploying a [Service](/reference/primitives/app/service) behind a load balancer requires a health check to determine whether a given [Process](/reference/primitives/app/process) is ready to handle requests.

Health checks must return a valid HTTP response code (200-399) within the configured `timeout`.

[Processes](/reference/primitives/app/process) that fail two health checks in a row are assumed dead and will be terminated and replaced.

## Definition

### Simple
```html
services:
  web:
    health: /check
```
> Specifying `health` as a string will set the `path` and leave the other options as defaults.

### Advanced

```html
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
| **grace**    | 5       | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | The HTTP endpoint that will be requested                                         |
| **timeout**  | 4       | The number of seconds to wait for a valid response                               |

## Liveness Checks

Liveness checks complement health checks by monitoring the ongoing health of running processes. While health checks (readiness probes) determine when a service is ready to receive traffic, liveness checks determine when a service should be restarted if it becomes unresponsive or enters a broken state.

When a liveness check fails, Kubernetes will restart the container, which can help recover from deadlocks, memory leaks, or other issues that cause a process to become unresponsive while still appearing to be running.

### Liveness Check Configuration

```html
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
```html
services:
  worker:
    liveness:
      path: /worker/health
      grace: 30
      interval: 10
      failureThreshold: 5
```

**Monitoring Memory-Intensive Applications:**
```html
services:
  processor:
    liveness:
      path: /memory-check
      grace: 45
      interval: 15
      timeout: 10
      failureThreshold: 3
```

## gRPC Health Checks

For services that use gRPC instead of HTTP, Convox provides support for gRPC health checks through the gRPC health checking protocol. To enable gRPC health checks, you need to:

1. Specify that your service uses the gRPC protocol in the port definition
2. Enable the gRPC health check with the `grpcHealthEnabled` attribute

### Basic Configuration

```html
services:
  api:
    build: .
    port: grpc:50051
    grpcHealthEnabled: true
```

### Advanced Configuration

You can customize the gRPC health check behavior using the same `health` attributes as HTTP health checks:

```html
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
| **grace**    | 5       | The amount of time in seconds to wait for a [Process](/reference/primitives/app/process) to boot before beginning health checks |
| **interval** | 5       | The number of seconds between health checks                                      |
| **path**     | /       | The service name to check within your gRPC health implementation                 |
| **timeout**  | 4       | The number of seconds to wait for a valid response                               |

### Implementation Requirements

Services using gRPC health checks must implement the gRPC Health Checking Protocol, which is defined in the [gRPC health checking protocol repository](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).

This protocol requires your service to implement a `Health` service with a `Check` method that returns the service's health status.

### Probe Behavior

When `grpcHealthEnabled` is set to `true`, Convox configures both:

1. A **readinessProbe** - Determines whether the service is ready to receive traffic
2. A **livenessProbe** - Determines whether the service should be restarted

The readinessProbe ensures that gRPC services won't receive traffic until they are fully ready, while the livenessProbe monitors the ongoing health of the service and initiates restarts if necessary.

Both probes use the health settings defined in your `convox.yml`, ensuring consistent behavior throughout the service lifecycle.

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