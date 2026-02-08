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
- Supports HTTP/1.1, HTTP/2 (h2c), and gRPC
- **Supports TCP connections via GCP SSH Bastion (for databases)**
- Automatic local port assignment (no collisions)
- Single fixed entry port for HTTP/gRPC, dedicated ports for TCP
- **Individual listener port for gRPC services** (`listener_port`)
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
- **GCP SSH Bastion (optional)**: `gcloud` CLI and Application Default Credentials (ADC) for database connections via IAP tunnel

> **Note:** kubectl-localmesh uses WebSocket-based port-forwarding, which requires Kubernetes 1.30 or later. SPDY-based port-forwarding (used in Kubernetes 1.29 and earlier) is not supported.

macOS example:

```bash
brew install envoy

# For GCP SSH Bastion support (optional):
# Install gcloud CLI: https://cloud.google.com/sdk/docs/install
gcloud auth application-default login
```

> **macOS TCP Support:** When running multiple TCP services (e.g., multiple databases on port 5432), kubectl-localmesh automatically assigns loopback IP aliases (127.0.0.x) using `ifconfig lo0 alias`. This requires `sudo` and is automatically cleaned up on exit.

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
    # ssh_key_path: ~/.ssh/custom-key  # Optional: custom SSH key path (--experimental-ssh only)
    # ssh_user: my-user                # Optional: SSH username (--experimental-ssh only)

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

  # HTTP/2 Service (modern HTTP API)
  - kind: kubernetes
    host: api-v2.localhost
    namespace: api
    service: api-service
    port_name: http
    protocol: http2  # Explicitly use HTTP/2

  # gRPC Service with individual listener port
  - kind: kubernetes
    host: grpc-api.localhost
    namespace: grpc
    service: grpc-api
    protocol: grpc
    listener_port: 50051  # Listen on specific port instead of global listener_port

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
- `protocol`: `http`, `http2`, or `grpc`
  - `http`: HTTP/1.1 (default for most REST APIs)
  - `http2`: HTTP/2 cleartext (h2c) for HTTP/2-capable services
  - `grpc`: gRPC (requires HTTP/2)
- `listener_port`: (optional) Port for individual Envoy listener
  - When specified, the service listens on this port instead of global `listener_port`
  - Useful for gRPC clients that require specific ports (e.g., `grpcurl host:50051`)

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

### Validate configuration

You can validate configuration files before running:

```bash
# Go-level validation (same checks as 'up')
kubectl localmesh validate -f services.yaml

# Additionally validate against JSON Schema (detects typos, unknown fields)
kubectl localmesh validate -f services.yaml --strict
```

### Subcommands

- `up`: Start the local service mesh
- `validate`: Validate configuration file
- `dump-envoy-config`: Dump Envoy configuration to stdout
- `down`: Stop the running mesh (planned)
- `status`: Show mesh status (planned)

### Global Flags

The following flags are available for all subcommands:

- `--log-level string`: Log level for Envoy and internal operations (debug|info|warn, default: info)

### `up` Subcommand Flags

- `--experimental-ssh`: [experimental] Use Go SDK for SSH tunnel instead of `gcloud` CLI. This uses IAP TCP Tunnel + SSH implemented in pure Go, removing the `gcloud` CLI dependency for SSH bastion connections. This feature is experimental and may have issues with OS Login username resolution and SSH authentication.

Examples:

```bash
# Debug mode for all subcommands
kubectl localmesh --log-level debug up -f services.yaml
kubectl localmesh --log-level debug dump-envoy-config -f services.yaml

# Use experimental Go SDK SSH tunnel
sudo kubectl localmesh up -f services.yaml --experimental-ssh
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
- gRPC (with `listener_port`): `grpcurl -plaintext grpc-api.localhost:50051 list`
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

### Protocol Selection Guide

kubectl-localmesh supports three protocol options for Kubernetes services:

- **`protocol: http`** (default)
  - Use for standard REST APIs that only support HTTP/1.1
  - Most compatible option
  - Example: Traditional web services, legacy APIs

- **`protocol: http2`**
  - Use for services that explicitly support HTTP/2
  - Better performance with multiplexing
  - Example: Modern HTTP APIs optimized for HTTP/2
  - Note: Service must support HTTP/2 cleartext (h2c)

- **`protocol: grpc`**
  - Use for gRPC services
  - Requires HTTP/2 (automatically configured)
  - Example: gRPC microservices

**How to choose:**
1. If the service is gRPC → use `protocol: grpc`
2. If the service supports HTTP/2 → use `protocol: http2` for better performance
3. If unsure or service only supports HTTP/1.1 → use `protocol: http` (default)

**Troubleshooting:**
- If you get "502 Bad Gateway" with `protocol error`, the service likely only supports HTTP/1.1
  - Solution: Change to `protocol: http`
- If gRPC calls fail, ensure you're using `protocol: grpc` (not `http2`)

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

### JSON Schema for Editor Integration

A JSON Schema file (`schemas/config.schema.json`) is provided for editor support (autocompletion and inline validation).

**For files inside the repository:**

```yaml
# yaml-language-server: $schema=schemas/config.schema.json
```

**For files outside the repository** (e.g., your own `services.yaml`):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/usadamasa/kubectl-local-mesh/main/schemas/config.schema.json
```

To pin to a specific version:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/usadamasa/kubectl-local-mesh/v0.3.0/schemas/config.schema.json
```

This works with VS Code (YAML extension), IntelliJ IDEA, and other editors that support `yaml-language-server`.

The `--strict` flag in the `validate` subcommand uses this same schema to detect typos and unknown fields that Go-level validation does not catch.

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
- ✅ **JSON Schema for configuration validation and editor integration**
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
