{
  description = "prometheus-render – render PromQL queries as PNG charts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "0.1.4";
    in
    flake-utils.lib.eachSystem [ "aarch64-linux" "x86_64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        pythonEnv = pkgs.python3.withPackages (ps: with ps; [
          requests
          matplotlib
        ]);

        prometheus-render = pkgs.writeShellScriptBin "prometheus-render" ''
          export PROMETHEUS_RENDER_VERSION="${version}"
          exec ${pythonEnv}/bin/python3 ${./prometheus_render.py} "$@"
        '';
      in
      {
        packages.default = prometheus-render;

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pythonEnv
          ];
        };
      }
    ) // {
      nixosModules.default = import ./nixos/module.nix self;
    };
}
