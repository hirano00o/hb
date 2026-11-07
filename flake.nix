{
  description = "hb — Hatena Blog CLI tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGo127Module {
          pname = "hb";
          version = self.shortRev or "dev";
          src = self;
          subPackages = [ "cmd/hb" ];
          vendorHash = "sha256-RvQLb0KCnPszIuQutyjY26j41uYY9XNo73O5KOKA3ms=";
          ldflags = [ "-s" "-w" ];
          env.CGO_ENABLED = "0";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_27
            pkgs.gopls
            pkgs.golangci-lint
          ];
        };
      });
}
