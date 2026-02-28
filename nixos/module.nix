self:
{ config, lib, pkgs, ... }:

let
  cfg = config.programs.prometheus-renderer;
  renderPkg = self.packages.${pkgs.system}.default;
in
{
  options.programs.prometheus-renderer = {
    enable = lib.mkEnableOption "prometheus-render CLI tool for rendering PromQL queries as PNG charts";

    package = lib.mkOption {
      type = lib.types.package;
      default = renderPkg;
      description = "The prometheus-render package to use.";
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];
  };
}
