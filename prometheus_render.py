#!/usr/bin/env python3
"""Render Prometheus queries as PNG charts."""

import argparse
import re
import sys
import time
from datetime import datetime

import matplotlib
matplotlib.use("Agg")
import matplotlib.dates as mdates  # noqa: E402
import matplotlib.pyplot as plt  # noqa: E402
import requests  # noqa: E402


def parse_range(range_str: str) -> int:
    """Convert a human range string (e.g. '24h', '7d') to seconds."""
    m = re.fullmatch(r"(\d+)([smhdw])", range_str)
    if not m:
        print(
            f"Invalid range format: {range_str} (expected e.g. 1h, 24h, 7d)",
            file=sys.stderr,
        )
        sys.exit(1)
    value, unit = int(m.group(1)), m.group(2)
    multipliers = {"s": 1, "m": 60, "h": 3600, "d": 86400, "w": 604800}
    return value * multipliers[unit]


def metric_label(metric: dict) -> str:
    """Build a human-readable label from a Prometheus metric dict."""
    exclude = {"__name__", "job", "instance"}
    parts = [f"{k}={v}" for k, v in sorted(metric.items()) if k not in exclude]
    return ", ".join(parts) if parts else metric.get("__name__", "value")


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="prometheus-render",
        description="Render a PromQL query as a PNG chart.",
    )
    parser.add_argument(
        "--url",
        default="http://localhost:9090",
        help="Prometheus base URL (default: http://localhost:9090)",
    )
    parser.add_argument(
        "--query",
        required=True,
        help="PromQL query expression",
    )
    parser.add_argument(
        "--range",
        default="24h",
        dest="range_str",
        help="Time range (e.g. 1h, 24h, 7d; default: 24h)",
    )
    parser.add_argument(
        "--title",
        default="",
        help="Chart title",
    )
    parser.add_argument(
        "--output",
        required=True,
        help="Output PNG file path",
    )
    parser.add_argument(
        "--width",
        type=int,
        default=800,
        help="Image width in pixels (default: 800)",
    )
    parser.add_argument(
        "--height",
        type=int,
        default=300,
        help="Image height in pixels (default: 300)",
    )
    parser.add_argument(
        "--step",
        default=None,
        help="Query resolution step in seconds (default: auto, ~300 data points)",
    )
    args = parser.parse_args()

    range_seconds = parse_range(args.range_str)
    end = time.time()
    start = end - range_seconds
    step = args.step or str(max(1, range_seconds // 300))

    # Query Prometheus
    try:
        resp = requests.get(
            args.url.rstrip("/") + "/api/v1/query_range",
            params={
                "query": args.query,
                "start": start,
                "end": end,
                "step": step,
            },
            timeout=30,
        )
        resp.raise_for_status()
    except requests.RequestException as e:
        print(f"Error querying Prometheus: {e}", file=sys.stderr)
        sys.exit(1)

    data = resp.json()
    if data.get("status") != "success":
        print(
            f"Prometheus error: {data.get('error', 'unknown')}",
            file=sys.stderr,
        )
        sys.exit(1)

    results = data["data"]["result"]
    if not results:
        print(f"No data returned for query: {args.query}", file=sys.stderr)
        sys.exit(1)

    # Render chart
    dpi = 100
    fig, ax = plt.subplots(
        figsize=(args.width / dpi, args.height / dpi), dpi=dpi
    )

    for series in results:
        timestamps = [
            datetime.fromtimestamp(float(v[0])) for v in series["values"]
        ]
        values = [float(v[1]) for v in series["values"]]
        label = metric_label(series["metric"])
        ax.plot(timestamps, values, label=label, linewidth=1.5)

    date_fmt = "%H:%M" if range_seconds <= 86400 else "%m-%d"
    ax.xaxis.set_major_formatter(mdates.DateFormatter(date_fmt))
    fig.autofmt_xdate()

    if args.title:
        ax.set_title(args.title, fontsize=11, pad=6)

    if len(results) > 1:
        ax.legend(fontsize=8, loc="best")

    ax.grid(True, alpha=0.3)
    fig.tight_layout()

    try:
        fig.savefig(args.output, dpi=dpi, bbox_inches="tight")
    except Exception as e:
        print(f"Error writing PNG: {e}", file=sys.stderr)
        sys.exit(1)
    finally:
        plt.close(fig)


if __name__ == "__main__":
    main()
