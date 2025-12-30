# kubectl-localmesh

`kubectl-localmesh` is a **local-only pseudo service mesh** built on top of `kubectl port-forward`.

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

`kubectl-localmesh` provides an **ingress/gateway-like experience**, but:

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
|  http://users-api.localhost
|  grpc://billing-api.localhost
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
- Envoy listens on a single local port (default: `80`)

---

## Installation

```sh
go install github.com/jpeach/kubectl-localmesh@latest
kubectl localmesh --help
```

### Prerequisites

- `kubectl`
- Access to a **Kubernetes 1.30+** cluster (WebSocket port-forward support required)
- `envoy` installed locally
- Go 1.21+ (if building from source)

> **Note:** kubectl-localmesh uses WebSocket-based port-forwarding, which requires Kubernetes 1.30 or later. SPDY-based port-forwarding (used in Kubernetes 1.29 and earlier) is not supported.

macOS example:

```bash
brew install envoy
```

## Usage

### Configuration file

Create a services.yaml file:

```yaml
listener_port: 80
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

By default, kubectl-localmesh automatically updates `/etc/hosts`, which requires sudo:

```bash
sudo kubectl localmesh up -f services.yaml
```

Or use positional argument:

```bash
sudo kubectl localmesh up services.yaml
```

To disable automatic `/etc/hosts` update:

```bash
kubectl localmesh up -f services.yaml --no-edit-hosts
```

### Subcommands

- `up`: Start the local service mesh
- `dump-envoy-config`: Dump Envoy configuration to stdout
- `down`: Stop the running mesh (planned)
- `status`: Show mesh status (planned)

### Global Flags

The following flags are available for all subcommands:

- `--log-level string`: Log level for Envoy and internal operations (debug|info|warn, default: info)

Examples:

```bash
# Debug mode for all subcommands
kubectl localmesh --log-level debug up -f services.yaml
kubectl localmesh --log-level debug dump-envoy-config -f services.yaml
```

Example output:

```
/etc/hosts updated successfully
pf: users-api.localhost -> users/users-api:50051 via 127.0.0.1:43127
pf: billing-api.localhost -> billing/billing-api:8080 via 127.0.0.1:51234

envoy config: /tmp/kubectl-localmesh-XXXXXX/envoy.yaml
listen: 0.0.0.0:80
```

Access services

By default, `/etc/hosts` is automatically updated, enabling simple hostname-based access:

- HTTP: `curl http://billing-api.localhost/health`
- gRPC: `grpcurl -plaintext users-api.localhost list`

When using port 80 (set `listener_port: 80` in config):

- HTTP: `curl http://billing-api.localhost/health`
- gRPC: `grpcurl -plaintext users-api.localhost list`

No local port numbers to remember.
No conflicts to resolve.
No Host header required.

gRPC notes
- gRPC is supported over plaintext (h2c)
- Clients must allow non-TLS connections (e.g. grpcurl -plaintext)
- If your client requires TLS, Envoy can be configured for local TLS termination (future work)

### /etc/hosts Automatic Management

By default, kubectl-localmesh automatically updates `/etc/hosts` to enable simple hostname-based access without specifying the Host header.

**Default behavior (requires sudo):**

```bash
sudo kubectl localmesh up -f services.yaml
```

This automatically adds entries like:

```
127.0.0.1 users-api.localhost
127.0.0.1 billing-api.localhost
```

**Disable automatic /etc/hosts update:**

```bash
kubectl localmesh up -f services.yaml --no-edit-hosts

# In this case, you need to specify the Host header manually:
curl -H "Host: users-api.localhost" http://127.0.0.1:80/
```

**Cleanup:**

When you stop kubectl-localmesh (Ctrl+C), it automatically removes the managed entries from /etc/hosts.

### Advanced Usage

#### Dump Envoy Configuration

You can dump the generated Envoy configuration to stdout for debugging or inspection:

```bash
kubectl localmesh dump-envoy-config -f services.yaml
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
kubectl localmesh dump-envoy-config -f services.yaml --mock-config mocks.yaml
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
- ✅ Subcommands (`up` implemented, `down` and `status` planned)
- TLS support via local certificates
- gRPC-web support
- Envoy-less HTTP-only mode
- Config hot-reload
- Better status / diagnostics

---

Naming

kubectl-localmesh means:

- kubectl: kubectl-native workflow
- local: strictly local execution
- mesh: mesh-like routing behavior, not a real mesh

It is intentionally explicit about its scope.
