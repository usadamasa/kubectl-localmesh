# kubectl-local-mesh

`kubectl-local-mesh` is a **local-only pseudo service mesh** built on top of `kubectl port-forward`.

It lets you access multiple Kubernetes Services across namespaces through a single local entrypoint, with host-based routing for both HTTP and gRPC, without installing anything into your cluster.

This is designed for development, debugging, and local exploration of real clusters.

## Why this exists

If you have ever done this:

- Manually running multiple `kubectl port-forward` commands
- Forgetting which local port maps to which Service
- Hitting local port conflicts
- Wanting ingress-like routing without touching the cluster
- Needing to access gRPC services locally

This tool smooths that out.

`kubectl-local-mesh` provides an **ingress/gateway-like experience**, but:

- No Ingress
- No Service Mesh
- No CRDs
- No cluster-side installation
- Local process only

Think of it as a **shadow gateway** for your cluster.

---

## Key features

- Local-only (no cluster changes)
- Works across multiple namespaces
- Supports HTTP and gRPC (h2c / plaintext)
- Automatic local port assignment (no collisions)
- Single fixed entry port
- Host-based routing (`<service>.localhost`)
- Auto-reconnecting `port-forward`
- kubectl-native UX (krew plugin friendly)

---

## How it works (conceptually)

```
[ client ]
|
|  http://users-api.localhost:18080
|  grpc://billing-api.localhost:18080
v
[ local Envoy ]
|
|  (random local ports)
v
[kubectl port-forward]
|
v
[Kubernetes Services]
```

- Each Service gets its own `kubectl port-forward`
- Local ports are dynamically allocated
- Envoy routes traffic by `Host` / `:authority`
- Envoy listens on a single local port (default: `18080`)

---

## Installation

### Prerequisites

- `kubectl`
- Access to a Kubernetes cluster
- `envoy` installed locally
- Go 1.21+ (if building from source)

macOS example:

```bash
brew install envoy
```

## Usage

### Configuration file

Create a services.yaml file:

```yaml
listener_port: 18080
services:
  - host: users-api.localhost
    namespace: users
    service: users-api
    port_name: grpc
    type: grpc

  - host: billing-api.localhost
    namespace: billing
    service: billing-api
    port_name: http
    type: http

  - host: admin.localhost
    namespace: admin
    service: admin-web
    port: 8080
    type: http
```

Notes:
- host is the local access hostname
- namespace and service refer to the Kubernetes Service
- Use port_name if the Service has multiple ports
- port can be used as a fallback for explicit control
- type is currently informational (HTTP / gRPC)

### Run

```
kubectl local-mesh -f services.yaml
```

Or directly:

```
kubectl-local-mesh services.yaml
```

Example output:

```
pf: users-api.localhost -> users/users-api:50051 via 127.0.0.1:43127
pf: billing-api.localhost -> billing/billing-api:8080 via 127.0.0.1:51234

listen: 0.0.0.0:18080
```

Access services

- HTTP: `curl http://billing-api.localhost:18080/health`
- gRPC: `grpcurl -plaintext users-api.localhost:18080 list`

No local port numbers to remember.
No conflicts to resolve.

gRPC notes
- gRPC is supported over plaintext (h2c)
- Clients must allow non-TLS connections (e.g. grpcurl -plaintext)
- If your client requires TLS, Envoy can be configured for local TLS termination (future work)

### Advanced Usage

#### Dump Envoy Configuration

You can dump the generated Envoy configuration to stdout for debugging or inspection:

```bash
kubectl-local-mesh --dump-envoy-config -f services.yaml

# Save to file
kubectl-local-mesh --dump-envoy-config -f services.yaml > envoy-config.yaml
```

This is useful for:
- Understanding the generated Envoy configuration
- Debugging routing issues
- Learning Envoy configuration patterns

#### Offline Mode (Mock Configuration)

You can generate Envoy configuration without connecting to a Kubernetes cluster by using a mock configuration file:

```bash
# Create a mock configuration file
cat > mocks.yaml <<EOF
mocks:
  - namespace: users
    service: users-api
    port_name: grpc
    resolved_port: 50051
  - namespace: billing
    service: billing-api
    port_name: http
    resolved_port: 8080
  - namespace: admin
    service: admin-web
    port_name: ""
    resolved_port: 8080
EOF

# Dump config using mocks (no cluster connection required)
kubectl-local-mesh --dump-envoy-config -f services.yaml --mock-config mocks.yaml
```

This is useful for:
- Testing configuration changes without cluster access
- CI/CD pipelines
- Offline development

---

What this is NOT

This tool intentionally does not:
- Replace a real Service Mesh
- Provide mTLS
- Modify cluster networking
- Expose services externally
- Support production traffic

It is for local development and debugging only.

---

Design philosophy
- Prefer kubectl primitives over cluster-side components
- Keep failure modes obvious
- Make it easy to start and easy to throw away
- Match real ingress/gateway concepts where possible
- Optimize for developer ergonomics, not completeness

---

Roadmap ideas
- krew distribution
- Subcommands (up, down, status)
- TLS support via local certificates
- gRPC-web support
- Envoy-less HTTP-only mode
- Config hot-reload
- Better status / diagnostics

---

Naming

kubectl-local-mesh means:
- kubectl: kubectl-native workflow
- local: strictly local execution
- mesh: mesh-like routing behavior, not a real mesh

It is intentionally explicit about its scope.
