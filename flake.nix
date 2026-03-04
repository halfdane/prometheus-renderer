{
  description = "prometheus-render – render PromQL queries as PNG charts";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "aarch64-linux" "x86_64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default =
          let renderVersion = "0.2.0"; in
          pkgs.buildGoModule {
            pname = "prometheus-render";
            version = renderVersion;
            src = ./.;
            # Run `nix build` once after `go mod tidy`; Nix will print the
            # correct hash in the error – paste it here.
            vendorHash = null;
            ldflags = [ "-X main.version=v${renderVersion}" ];
            meta = {
              description = "Render PromQL queries as PNG charts";
              mainProgram = "prometheus-render";
            };
          };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools # staticcheck
          ];
        };
      }
    ) // {
      nixosModules.default = import ./nixos/module.nix self;
    };
}
