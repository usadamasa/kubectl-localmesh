{
  description = "Development environment for kubectl-localmesh (Go, lint, and release tools)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # go.mod の go ディレクティブと同じ版。ずれたら GOTOOLCHAIN=local で即座に失敗させる
      # nixos-unstable の go_1_27 は 1.27.3 で、net/http (CVE-2026-97054) と cmd/go・cmd/cgo 等
      # (CVE-2026-97033〜97043) の脆弱性が残る (1.27.4 で修正)。
      # goreleaser もこの go で配布バイナリをビルドするので、channel の go_1_27 が 1.27.4 より古い間だけ
      # go.dev の配布バイナリに差し替える (ソースからビルドすると binary cache に無く、CI の job ごとにビルドが走る)。
      # Dependabot の nix PR が 1.27.4 以上の nixpkgs を取り込んだ時点で channel の go_1_27 に戻る。
      # 戻った後の差し替えの分岐は使われないので、そのとき devGo ごと消す
      devGo =
        pkgs:
        if pkgs.lib.versionOlder pkgs.go_1_27.version "1.27.4" then
          pkgs.stdenvNoCC.mkDerivation rec {
            pname = "go-bin";
            version = "1.27.4";
            src =
              let
                platform = with pkgs.stdenvNoCC.hostPlatform.go; "${GOOS}-${GOARCH}";
              in
              pkgs.fetchurl {
                url = "https://go.dev/dl/go${version}.${platform}.tar.gz";
                # https://go.dev/dl/?mode=json&include=all の sha256
                sha256 =
                  {
                    darwin-arm64 = "91a44a2b8a716600ad94a4f143e156116620db8828086a6e3602291be9bd0bb4";
                    linux-amd64 = "7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840";
                    linux-arm64 = "3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92";
                  }
                  .${platform};
              };
            # strip は darwin の署名を壊す。配布物をそのまま置く
            dontFixup = true;
            installPhase = ''
              mkdir -p $out/share $out/bin
              cp -r . $out/share/go
              ln -s $out/share/go/bin/go $out/share/go/bin/gofmt $out/bin/
            '';
          }
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
