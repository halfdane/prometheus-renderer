package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"time"

	renderchart "github.com/halfdane/prometheus-renderer/internal/chart"
	"github.com/halfdane/prometheus-renderer/internal/promclient"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

const usage = `prometheus-render – render PromQL queries as PNG charts

Usage:
  prometheus-render [flags]

Flags:
  --url           Prometheus base URL (default: http://localhost:9090)
  --query         PromQL query expression (required)
  --range         Time range, e.g. 1h, 24h, 7d (default: 24h)
  --title         Chart title (optional)
  --output        Output PNG file path (required)
  --width         Image width in pixels (default: 800)
  --height        Image height in pixels (default: 300)
  --step          Resolution step in seconds; auto-computed when omitted
  --vlines-query  PromQL query for event markers: the first timestamp of each
                  returned series is drawn as a vertical line
  --light         Use a light color scheme (default: dark)
  --version       Print version and exit
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("prometheus-render", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var (
		rawURL       string
		query        string
		rangeStr     string
		title        string
		output       string
		width        int
		height       int
		step         string
		vlinesQuery  string
		light        bool
		showVersion  bool
	)

	fs.StringVar(&rawURL, "url", "http://localhost:9090", "Prometheus base URL")
	fs.StringVar(&query, "query", "", "PromQL query expression")
	fs.StringVar(&rangeStr, "range", "24h", "Time range (e.g. 1h, 24h, 7d)")
	fs.StringVar(&title, "title", "", "Chart title")
	fs.StringVar(&output, "output", "", "Output PNG file path")
	fs.IntVar(&width, "width", 800, "Image width in pixels")
	fs.IntVar(&height, "height", 300, "Image height in pixels")
	fs.StringVar(&step, "step", "", "Resolution step in seconds (default: auto)")
	fs.StringVar(&vlinesQuery, "vlines-query", "", "PromQL query for vertical event markers")
	fs.BoolVar(&light, "light", false, "Use light color scheme")
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if showVersion {
		fmt.Printf("prometheus-render %s\n", version)
		return nil
	}

	if query == "" {
		return fmt.Errorf("--query is required")
	}
	if output == "" {
		return fmt.Errorf("--output is required")
	}

	rangeSeconds, err := parseRange(rangeStr)
	if err != nil {
		return err
	}

	resolvedStep := step
	if resolvedStep == "" {
		// Target ~300 data points, matching the Python implementation.
		resolvedStep = strconv.Itoa(int(math.Max(1, float64(rangeSeconds)/300)))
	}

	now := time.Now()
	start := now.Add(-time.Duration(rangeSeconds) * time.Second)

	client := promclient.New(rawURL)
	ctx := context.Background()

	params := promclient.QueryRangeParams{
		Query: query,
		Start: start,
		End:   now,
		Step:  resolvedStep,
	}

	series, err := client.QueryRange(ctx, params)
	if err != nil {
		return fmt.Errorf("query Prometheus: %w", err)
	}
	if len(series) == 0 {
		return fmt.Errorf("no data returned for query: %s", query)
	}

	var vlines []promclient.Series
	if vlinesQuery != "" {
		fmt.Fprintln(os.Stderr, "warning: --vlines-query is not supported with the current chart backend and will be ignored")
		vlines, err = client.QueryRange(ctx, promclient.QueryRangeParams{
			Query: vlinesQuery,
			Start: start,
			End:   now,
			Step:  resolvedStep,
		})
		if err != nil {
			// Non-fatal: warn and continue without vlines.
			fmt.Fprintf(os.Stderr, "warning: could not fetch vlines query: %v\n", err)
		}
	}

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	opts := renderchart.Options{
		Width:        width,
		Height:       height,
		Title:        title,
		Light:        light,
		RangeSeconds: rangeSeconds,
	}

	if err := renderchart.Render(series, vlines, opts, f); err != nil {
		return fmt.Errorf("render chart: %w", err)
	}

	return nil
}

var rangeRe = regexp.MustCompile(`^(\d+)([smhdw])$`)

// parseRange converts a human range string (e.g. "24h", "7d") to seconds.
func parseRange(s string) (int, error) {
	m := rangeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid range %q: expected format like 1h, 24h, 7d", s)
	}
	n, _ := strconv.Atoi(m[1])
	multipliers := map[string]int{
		"s": 1,
		"m": 60,
		"h": 3600,
		"d": 86400,
		"w": 604800,
	}
	return n * multipliers[m[2]], nil
}
