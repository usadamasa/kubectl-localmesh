# Changelog

## [v0.2.0](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.7...v0.2.0) - 2026-01-03
### New Features 🎉
- support db via bastion by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/46

## [v0.2.0](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.7...v0.2.0)

### Breaking Changes 🛠

**Configuration file format has changed.** The `type` field has been replaced with `kind` and `protocol` fields for clearer service type distinction.

#### What Changed

- **Service type indication**: Changed from `type` field to `kind` field
- **Kubernetes services**: The `type: http/grpc` is now split into `kind: kubernetes` + `protocol: http/grpc`
- **TCP services**: Changed from `type: tcp` to `kind: tcp`

#### Migration Steps

**For Kubernetes Services (HTTP/gRPC):**

Old format (v0.1.x):
```yaml
services:
  - host: users-api.localhost
    namespace: users
    service: users-api
    type: grpc  # OLD
```

New format (v0.2.0+):
```yaml
services:
  - kind: kubernetes  # NEW: explicit kind field
    host: users-api.localhost
    namespace: users
    service: users-api
    protocol: grpc  # NEW: separate protocol field
```

**For TCP Services (SSH Bastion):**

Old format (v0.1.x):
```yaml
services:
  - host: db.localhost
    type: tcp  # OLD
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
```

New format (v0.2.0+):
```yaml
services:
  - kind: tcp  # NEW: explicit kind field
    host: db.localhost
    ssh_bastion: primary
    target_host: 10.0.0.1
    target_port: 5432
```

#### Why This Change?

- **Type Safety**: Clear distinction between service kinds at the type level
- **Better Validation**: Kind-specific validation rules
- **Clearer Semantics**: `kind` distinguishes the service mechanism, `protocol` distinguishes HTTP vs gRPC
- **Extensibility**: Easier to add new service kinds in the future

### New Features 🎉
- feat: implement tagged union type for service configuration by @usadamasa

## [v0.1.7](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.6...v0.1.7) - 2025-12-30
### New Features 🎉
- feat: introduce Cobra-based subcommand structure with 'up' command by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/42
- refactor: reorganize CLI options and introduce dump-envoy-config subcommand by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/45

## [v0.1.6](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.5...v0.1.6) - 2025-12-30
### Bug Fixes 🐛
- Bugfix/handle invalid hosts by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/41
### Other Changes
- migrate to kubernetes/client-go from kubectl by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/36

## [v0.1.5](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.4...v0.1.5) - 2025-12-29
### Breaking Changes 🛠
- refactor: rename project from kubectl-local-mesh to kubectl-localmesh by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/33

## [v0.1.4](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.3...v0.1.4) - 2025-12-29
### Other Changes
- now by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/30

## [v0.1.3](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.2...v0.1.3) - 2025-12-29
### Bug Fixes 🐛
- fix: /etc/hostsの空行累積問題を修正 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/28
- adopt kubectl plugin naming by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/29

## [v0.1.2](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.1...v0.1.2) - 2025-12-29
### Bug Fixes 🐛
- bugfix: fix with golangci-lint by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/17

## [v0.1.1](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.0...v0.1.1) - 2025-12-28
- setup ci by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/11
- introduce tagpr by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/12
- run tagpr with gh app token by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/14

## [v0.1.0](https://github.com/usadamasa/kubectl-localmesh/commits/v0.1.0) - 2025-12-27
- [from now] 2025/12/27 17:58:16 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/2
- [from now] 2025/12/27 21:59:49 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/3
- Make --update-hosts default to true for normal startup by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/4
- Change default listen port by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/6
- add ci-status-check by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/7
