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
      # nixos-unstable の go_1_27 は 1.27.3 で、net/http (CVE-2026-97054) と cmd/go・cmd/cgo 等
      # (CVE-2026-97033〜97043) の脆弱性が残る (1.27.4 で修正)。
      # goreleaser もこの go で配布バイナリをビルドするので、channel の go_1_27 が 1.27.4 より古い間だけソースを差し替える。
      # nixpkgs が追いつけば差し替えは自動で外れる (Dependabot の nix PR でも同じ)。外れた後は devGo ごと消してよい
      devGo =
        pkgs:
        if nixpkgs.lib.versionOlder pkgs.go_1_27.version "1.27.4" then
          pkgs.go_1_27.overrideAttrs (old: rec {
            version = "1.27.4";
            src = pkgs.fetchurl {
              url = "https://go.dev/dl/go${version}.src.tar.gz";
              hash = "sha256-c8+4VxrsDAEXdofbhxY9Rqog6XeWcncokaoXZU2aYJk=";
            };
          })
        else
          pkgs.go_1_27;
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
