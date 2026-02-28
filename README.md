# prometheus-renderer

Render Prometheus queries as PNG charts. Designed to be called on-demand by
tools like [ilias](https://github.com/halfdane/ilias) or scripts that send
reports to ntfy.sh/Telegram.

## Usage

```bash
prometheus-render \
  --url http://localhost:9090 \
  --query 'rate(node_cpu_seconds_total{mode!="idle"}[5m])' \
  --range 24h \
  --title "CPU Usage (24h)" \
  --output /tmp/cpu.png
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | `http://localhost:9090` | Prometheus base URL |
| `--query` | *(required)* | PromQL query expression |
| `--range` | `24h` | Time range (`1h`, `24h`, `7d`, etc.) |
| `--title` | *(empty)* | Chart title |
| `--output` | *(required)* | Output PNG file path |
| `--width` | `800` | Image width in pixels |
| `--height` | `300` | Image height in pixels |
| `--step` | *auto* | Resolution step in seconds |

## NixOS Module

Add to your flake inputs:

```nix
prometheus-renderer.url = "github:halfdane/prometheus-renderer";
prometheus-renderer.inputs.nixpkgs.follows = "nixpkgs";
```

Include the module and enable:

```nix
# in nixosModules list:
prometheus-renderer.nixosModules.default

# in host configuration:
programs.prometheus-renderer.enable = true;
```

This puts `prometheus-render` on the system PATH.

## Example: ilias integration

In your ilias `config.yaml`, use shell checks to generate charts:

```yaml
tiles:
  - title: CPU Usage
    checks:
      - name: chart
        command: prometheus-render --query 'rate(node_cpu_seconds_total{mode!="idle"}[5m])' --range 24h --title "CPU" --output /var/www/ilias/cpu.png
```

## License

MIT
