{
  description = "Development environment for kubectl-localmesh (Go, lint, and release tools)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # go.mod の go ディレクティブと同じ版。ずれたら GOTOOLCHAIN=local で即座に失敗させる
      # nixos-unstable の go_1_26 は 1.26.9 で、crypto/x509 の脆弱性 (GO-2026-6809、1.26.10 で修正) が残る。
      # goreleaser もこの go で配布バイナリをビルドするので、channel の go_1_26 が 1.26.10 より古い間だけソースを差し替える。
      # nixpkgs が追いつけば差し替えは自動で外れる (Dependabot の nix PR でも同じ)。外れた後は devGo ごと消してよい
      devGo =
        pkgs:
        if nixpkgs.lib.versionOlder pkgs.go_1_26.version "1.26.10" then
          pkgs.go_1_26.overrideAttrs (old: rec {
            version = "1.26.10";
            src = pkgs.fetchurl {
              url = "https://go.dev/dl/go${version}.src.tar.gz";
              hash = "sha256-/WPog7ovK/ypPBZGsx6+9j4rHk0rMXyXidl2po2B828=";
            };
          })
        else
          pkgs.go_1_26;
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShellNoCC {
          packages = with pkgs; [
            (devGo pkgs)
            go-task
            golangci-lint
            goreleaser
            yamllint
          ];
          env.GOTOOLCHAIN = "local";
        };
      });
    };
}
