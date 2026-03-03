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
        packages.default = pkgs.buildGo125Module {
          pname = "hb";
          version = self.shortRev or "dev";
          src = self;
          subPackages = [ "cmd/hb" ];
          vendorHash = "sha256-TDi1c8S/KSOYaKxKoMFhSmTtrfaF0M+i0fpiiHIcLYQ=";
          ldflags = [ "-s" "-w" ];
          env.CGO_ENABLED = "0";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_25
            pkgs.gopls
            pkgs.golangci-lint
          ];
        };
      });
}
