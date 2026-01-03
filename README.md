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
- **Supports TCP connections via GCP SSH Bastion (for databases)**
- Automatic local port assignment (no collisions)
- Single fixed entry port for HTTP/gRPC, dedicated ports for TCP
- Host-based routing (`<service>.localhost`)
- Auto-reconnecting `port-forward` and SSH tunnels
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
go install github.com/usadamasa/kubectl-localmesh@latest
kubectl localmesh --help
```

### Prerequisites

- `kubectl`
- Access to a **Kubernetes 1.30+** cluster (WebSocket port-forward support required)
- `envoy` installed locally
- Go 1.21+ (if building from source)
- **GCP SSH Bastion (optional)**: `gcloud` CLI and Application Default Credentials for database connections via SSH tunnel

> **Note:** kubectl-localmesh uses WebSocket-based port-forwarding, which requires Kubernetes 1.30 or later. SPDY-based port-forwarding (used in Kubernetes 1.29 and earlier) is not supported.

macOS example:

```bash
brew install envoy

# For GCP SSH Bastion support (optional):
# Install gcloud CLI: https://cloud.google.com/sdk/docs/install
gcloud auth application-default login
```

## Usage

### Configuration file

Create a services.yaml file:

```yaml
listener_port: 80

# Optional: GCP SSH Bastions for database connections
ssh_bastions:
  primary:
    instance: bastion-instance-1
    zone: asia-northeast1-a
    project: my-gcp-project

services:
  # Kubernetes Services (HTTP/gRPC)
  - kind: kubernetes
    host: users-api.localhost
    namespace: users
    service: users-api
    port_name: grpc
    protocol: grpc

  - kind: kubernetes
    host: billing-api.localhost
    namespace: billing
    service: billing-api
    port_name: http
    protocol: http

  - kind: kubernetes
    host: admin.localhost
    namespace: admin
    service: admin-web
    port: 8080
    protocol: http

  # Database via GCP SSH Bastion (TCP)
  - kind: tcp
    host: users-db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1  # Private IP (e.g., Cloud SQL)
    target_port: 5432
```

**Notes:**

**For Kubernetes Services:**
- `kind`: Must be `kubernetes`
- `host`: Local access hostname
- `namespace` and `service`: Kubernetes Service reference
- `port_name`: Used if the Service has multiple ports
- `port`: Explicit port number (fallback)
- `protocol`: `http` or `grpc`

**For Database via SSH Bastion:**
- `kind`: Must be `tcp`
- `host`: Local access hostname
- `ssh_bastion`: Reference to a defined SSH bastion
- `target_host`: Target database IP (private IP accessible from bastion)
- `target_port`: Target database port

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
- **Database (TCP)**: `psql -h users-db.localhost -p 5432 -U myuser`

When using port 80 (set `listener_port: 80` in config):

- HTTP: `curl http://billing-api.localhost/health`
- gRPC: `grpcurl -plaintext users-api.localhost list`

No local port numbers to remember (for HTTP/gRPC).
No conflicts to resolve.
No Host header required.

**TCP database connections:**
- Each TCP service (database) uses its own dedicated port (defined by `target_port`)
- Example: PostgreSQL on port 5432, MySQL on port 3306
- Access via hostname:port (e.g., `users-db.localhost:5432`)

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

## Breaking Changes (v0.2.0)

### Configuration File Format Change

**Version 0.2.0 introduces a breaking change in the configuration file format.** The old `type` field has been replaced with a `kind` field for clearer service type distinction.

### Migration Guide

#### Old Format (v0.1.x):

```yaml
services:
  # Kubernetes Service
  - host: users-api.localhost
    namespace: users
    service: users-api
    type: grpc  # OLD: combined type field

  # TCP Service via SSH Bastion
  - host: db.localhost
    type: tcp  # OLD: same 'type' field for different concepts
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
```

#### New Format (v0.2.0+):

```yaml
services:
  # Kubernetes Service
  - kind: kubernetes  # NEW: explicit kind field
    host: users-api.localhost
    namespace: users
    service: users-api
    protocol: grpc  # NEW: separate protocol field

  # TCP Service via SSH Bastion
  - kind: tcp  # NEW: explicit kind field
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
```

### Migration Steps:

1. **For Kubernetes Services (HTTP/gRPC)**:
   - Add `kind: kubernetes` field
   - Rename `type: http/grpc` to `protocol: http/grpc`

2. **For TCP Services (SSH Bastion)**:
   - Change `type: tcp` to `kind: tcp`
   - Other fields remain the same

### Why This Change?

The new format provides:
- **Type Safety**: Clear distinction between service kinds at the type level
- **Better Validation**: Kind-specific validation rules
- **Clearer Semantics**: `kind` distinguishes the service mechanism, `protocol` distinguishes HTTP vs gRPC

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
- ✅ Subcommands (`up` and `dump-envoy-config` implemented, `down` and `status` planned)
- ✅ **GCP SSH Bastion support for database connections (TCP proxy)**
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
