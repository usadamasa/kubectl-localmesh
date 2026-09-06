# Changelog

## [v0.4.1](https://github.com/usadamasa/kubectl-localmesh/compare/v0.4.0...v0.4.1) - 2026-09-06

### New Features 🎉
- chore(deps): bump golang.org/x/crypto from 0.48.0 to 0.49.0 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/106
- chore(deps): bump golang.org/x/oauth2 from 0.35.0 to 0.36.0 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/107
- chore(deps): bump golang.org/x/crypto from 0.49.0 to 0.53.0 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/114
### Dependencies
- chore(deps): bump k8s.io/api from 0.35.1 to 0.35.2 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/101
- chore(deps): bump github.com/moby/spdystream from 0.5.0 to 0.5.1 in the go_modules group across 1 directory by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/109
- chore(deps): bump k8s.io/apimachinery from 0.35.2 to 0.36.2 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/102
- chore(deps): bump k8s.io/client-go from 0.35.1 to 0.36.2 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/103
### Other Changes
- update by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/98
- fix: update golangci-lint to v2.10.1 to resolve go1.26 panic by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/104
- chore: remove Weekly Repo Status Report by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/112
- chore: upgrade to Go 1.26 and golangci-lint v2.12.2 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/113
- fix(e2e): Pod の生成待ちを kubectl rollout status に置き換える by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/121
- chore(tagpr): App ID をワークフローに直書きして repository variable を不要にする by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/120

## [v0.4.0](https://github.com/usadamasa/kubectl-localmesh/compare/v0.3.4...v0.4.0) - 2026-02-15
### New Features 🎉
- chore(deps): bump golang.org/x/crypto from 0.47.0 to 0.48.0 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/86
- chore(deps): bump golang.org/x/oauth2 from 0.34.0 to 0.35.0 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/88
### Dependencies
- chore(deps): bump k8s.io/api from 0.35.0 to 0.35.1 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/84
- chore(deps): bump k8s.io/client-go from 0.35.0 to 0.35.1 by @dependabot[bot] in https://github.com/usadamasa/kubectl-localmesh/pull/85
### Other Changes
- feat: add experimental Go SDK SSH tunnel and improve gcp-ssh-tunnel skill by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/82

## [v0.3.4](https://github.com/usadamasa/kubectl-localmesh/compare/v0.3.3...v0.3.4) - 2026-02-08
### New Features 🎉
- feat: add multi-cluster support with per-service cluster override by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/76
- feat: add version command support and install task by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/78
### Bug Fixes 🐛
- feat: add ReadBuildInfo fallback for go install version output by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/80

## [v0.3.3](https://github.com/usadamasa/kubectl-localmesh/compare/v0.3.2...v0.3.3) - 2026-02-08
### New Features 🎉
- feat: add JSON Schema for config validation and validate subcommand by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/75
### Other Changes
- refactor: overwrite_listen_portsをlistener_portに変更 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/70
- feat: add E2E test environment using docker-compose and k3s by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/72
- refactor: move snapshot tests from testdata/envoy-snapshots/ to test/snapshot/ by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/73
- test: add gRPC E2E test using grpcurl by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/74

## [v0.3.2](https://github.com/usadamasa/kubectl-localmesh/compare/v0.3.1...v0.3.2) - 2026-01-18
### New Features 🎉
- feat: loopback IPエイリアスによるTCPサービスの同一ポート対応 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/66

## [v0.3.1](https://github.com/usadamasa/kubectl-localmesh/compare/v0.3.0...v0.3.1) - 2026-01-18
### Bug Fixes 🐛
- fix: Envoy domainsにhost:port形式を追加してgRPCクライアント互換性を改善 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/62
### Other Changes
- refactor: Portをジェネリック型制約に変更しキャスト削減 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/64

## [v0.3.0](https://github.com/usadamasa/kubectl-localmesh/compare/v0.2.1...v0.3.0) - 2026-01-17
### New Features 🎉
- feat: support overwrite_listen_port by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/56
- feat: ログレベル階層化とユーザーフレンドリーなサマリー出力を実装 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/59
### Other Changes
- Refactor/switch kind by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/53
- feat: add CLI-based snapshot testing for Envoy configuration by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/55
- refactor: ポート番号にセマンティック型を導入 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/57
- refactor: dump/snapshotパッケージを分離して責務を明確化 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/58
- feat: Envoy警告抑止とGCP SSH tunnel IAP明示指定を追加 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/60

## [v0.2.1](https://github.com/usadamasa/kubectl-localmesh/compare/v0.2.0...v0.2.1) - 2026-01-11
### Bug Fixes 🐛
- bugfix: suport http1 by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/51
### Other Changes
- chore: add log by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/50

## [v0.2.0](https://github.com/usadamasa/kubectl-localmesh/compare/v0.1.7...v0.2.0) - 2026-01-03
### New Features 🎉
- support db via bastion by @usadamasa in https://github.com/usadamasa/kubectl-localmesh/pull/46

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
