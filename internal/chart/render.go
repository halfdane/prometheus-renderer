// Package chart renders Prometheus time-series data as PNG images using go-chart.
package chart

import (
	"fmt"
	"io"

	chart "github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"

	"github.com/halfdane/prometheus-renderer/internal/promclient"
)

// Options controls the chart size, appearance, and time range.
type Options struct {
	Width        int
	Height       int
	Title        string
	Light        bool // use light color scheme; default is dark
	RangeSeconds int  // determines the X axis time format
}

type colorScheme struct {
	lines  []drawing.Color
	bg     drawing.Color
	canvas drawing.Color
	text   drawing.Color
	axis   drawing.Color
	grid   drawing.Color
	vline  drawing.Color
}

// Catppuccin Mocha-inspired dark scheme.
var dark = colorScheme{
	lines: []drawing.Color{
		drawing.ColorFromHex("7dc4e4"),
		drawing.ColorFromHex("a6e3a1"),
		drawing.ColorFromHex("fab387"),
		drawing.ColorFromHex("cba6f7"),
		drawing.ColorFromHex("f38ba8"),
		drawing.ColorFromHex("89dceb"),
		drawing.ColorFromHex("f9e2af"),
		drawing.ColorFromHex("74c7ec"),
	},
	bg:     drawing.ColorFromHex("1e1e2e"),
	canvas: drawing.ColorFromHex("181825"),
	text:   drawing.ColorFromHex("cdd6f4"),
	axis:   drawing.ColorFromHex("6c7086"),
	grid:   drawing.ColorFromHex("313244"),
	vline:  drawing.Color{R: 243, G: 139, B: 168, A: 140},
}

// Catppuccin Latte-inspired light scheme.
var light = colorScheme{
	lines: []drawing.Color{
		drawing.ColorFromHex("1e66f5"),
		drawing.ColorFromHex("40a02b"),
		drawing.ColorFromHex("df8e1d"),
		drawing.ColorFromHex("8839ef"),
		drawing.ColorFromHex("d20f39"),
		drawing.ColorFromHex("04a5e5"),
		drawing.ColorFromHex("209fb5"),
		drawing.ColorFromHex("e64553"),
	},
	bg:     drawing.ColorFromHex("eff1f5"),
	canvas: drawing.ColorFromHex("e6e9ef"),
	text:   drawing.ColorFromHex("4c4f69"),
	axis:   drawing.ColorFromHex("8c8fa1"),
	grid:   drawing.ColorFromHex("bcc0cc"),
	vline:  drawing.Color{R: 210, G: 15, B: 57, A: 140},
}

// Render draws the chart to w as a PNG.
// series is the main data; vlines contains event-marker series (only the first
// timestamp of each is used to draw a vertical line).
func Render(series []promclient.Series, vlines []promclient.Series, opts Options, w io.Writer) error {
	if len(series) == 0 {
		return fmt.Errorf("no series to render")
	}

	cs := dark
	if opts.Light {
		cs = light
	}

	// Choose X axis time format based on the queried range.
	timeFormat := "15:04"
	if opts.RangeSeconds > 86400 {
		timeFormat = "01-02"
	}

	// Build the main line series.
	chartSeries := make([]chart.Series, 0, len(series)*2)
	for i, s := range series {
		lineColor := cs.lines[i%len(cs.lines)]
		ts := chart.TimeSeries{
			Name:    s.Label,
			XValues: s.Timestamps,
			YValues: s.Values,
			Style: chart.Style{
				StrokeColor: lineColor,
				StrokeWidth: 2,
				// Transparent fill so only the line is drawn.
				FillColor: drawing.Color{R: lineColor.R, G: lineColor.G, B: lineColor.B, A: 0},
			},
		}
		chartSeries = append(chartSeries, ts)
	}

	// For multi-series charts, annotate the last value of each line with its
	// label so the user can identify the series without a separate legend box.
	if len(series) > 1 {
		for i := range series {
			lineColor := cs.lines[i%len(cs.lines)]
			inner := chartSeries[i].(chart.TimeSeries)
			if len(inner.XValues) == 0 {
				continue
			}
			lastX := float64(inner.XValues[len(inner.XValues)-1].UnixNano())
			lastY := inner.YValues[len(inner.YValues)-1]
			chartSeries = append(chartSeries, chart.AnnotationSeries{
				Annotations: []chart.Value2{
					{Label: inner.Name, XValue: lastX, YValue: lastY},
				},
				Style: chart.Style{
					StrokeColor: lineColor,
					FontColor:   lineColor,
					FontSize:    7,
				},
			})
		}
	}

	// Map each vlines series to an XAxis GridLine at its first timestamp.
	vlineGridLines := buildVlineGridLines(vlines, cs.vline)

	c := chart.Chart{
		Width:  opts.Width,
		Height: opts.Height,
		Title:  opts.Title,
		TitleStyle: chart.Style{
			FontColor: cs.text,
			FontSize:  11,
		},
		Background: chart.Style{
			FillColor:   cs.bg,
			StrokeColor: cs.bg,
			Padding:     chart.Box{Top: 20, Left: 20, Right: 40, Bottom: 10},
		},
		Canvas: chart.Style{
			FillColor:   cs.canvas,
			StrokeColor: cs.canvas,
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat(timeFormat),
			Style: chart.Style{
				FontColor:   cs.text,
				StrokeColor: cs.axis,
				FontSize:    8,
			},
			GridLines: vlineGridLines,
		},
		YAxis: chart.YAxis{
			Style: chart.Style{
				FontColor:   cs.text,
				StrokeColor: cs.axis,
				FontSize:    8,
			},
			GridMajorStyle: chart.Style{
				StrokeColor: cs.grid,
				StrokeWidth: 0.5,
			},
		},
		Series: chartSeries,
	}

	return c.Render(chart.PNG, w)
}

func buildVlineGridLines(vlines []promclient.Series, color drawing.Color) []chart.GridLine {
	if len(vlines) == 0 {
		return nil
	}
	lines := make([]chart.GridLine, 0, len(vlines))
	for _, vs := range vlines {
		if len(vs.Timestamps) == 0 {
			continue
		}
		lines = append(lines, chart.GridLine{
			Value: float64(vs.Timestamps[0].UnixNano()),
			Style: chart.Style{
				StrokeColor: color,
				StrokeWidth: 1,
			},
		})
	}
	return lines
}
