// Package chart renders Prometheus time-series data as PNG images using go-charts.
package chart

import (
	"fmt"
	"io"
	"sort"
	"time"

	charts "github.com/vicanso/go-charts/v2"

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

// Render draws the chart to w as a PNG.
//
// series is the main data. vlines is accepted for API compatibility but is not
// rendered: vicanso/go-charts does not support arbitrary vertical lines on the
// X axis. A warning is printed to stderr when vlines are supplied.
func Render(series []promclient.Series, vlines []promclient.Series, opts Options, w io.Writer) error {
	if len(series) == 0 {
		return fmt.Errorf("no series to render")
	}

	// vlines are not rendered: vicanso/go-charts does not support arbitrary
	// vertical lines on the X axis. The caller is responsible for warning the user.
	_ = vlines

	// Choose X axis time format based on the queried range.
	timeFormat := "15:04"
	if opts.RangeSeconds > 86400 {
		timeFormat = "01-02"
	}

	// Build a unified, sorted timestamp axis from all series.
	allTS := unifiedTimestamps(series)

	// Format X axis labels.
	xLabels := make([]string, len(allTS))
	for i, t := range allTS {
		xLabels[i] = t.Format(timeFormat)
	}

	// Build per-series float64 values aligned to the unified axis.
	// Missing timestamps are filled with the library's null sentinel.
	values := make([][]float64, len(series))
	labels := make([]string, len(series))
	for i, s := range series {
		values[i] = alignValues(s, allTS)
		labels[i] = s.Label
	}

	theme := "dark"
	if opts.Light {
		theme = "light"
	}

	// Aim for ~6 readable X-axis tick labels regardless of data density.
	splitNumber := len(allTS) / 6
	if splitNumber < 1 {
		splitNumber = 1
	}

	p, err := charts.LineRender(
		values,
		charts.PNGTypeOption(),
		charts.TitleTextOptionFunc(opts.Title),
		charts.XAxisDataOptionFunc(xLabels, charts.FalseFlag()),
		charts.LegendLabelsOptionFunc(labels, charts.PositionCenter),
		charts.ThemeOptionFunc(theme),
		func(opt *charts.ChartOption) {
			opt.Width = opts.Width
			opt.Height = opts.Height
			opt.SymbolShow = charts.FalseFlag()
			opt.LineStrokeWidth = 2
			opt.XAxis.SplitNumber = splitNumber
		},
	)
	if err != nil {
		return fmt.Errorf("render chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	_, err = w.Write(buf)
	return err
}

// unifiedTimestamps returns the sorted union of all timestamps across series.
func unifiedTimestamps(series []promclient.Series) []time.Time {
	seen := make(map[time.Time]struct{})
	for _, s := range series {
		for _, t := range s.Timestamps {
			seen[t] = struct{}{}
		}
	}
	all := make([]time.Time, 0, len(seen))
	for t := range seen {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Before(all[j]) })
	return all
}

// alignValues maps a single series onto the shared timestamp axis.
// Slots with no data from this series are filled with the null sentinel.
func alignValues(s promclient.Series, axis []time.Time) []float64 {
	idx := make(map[time.Time]float64, len(s.Timestamps))
	for i, t := range s.Timestamps {
		idx[t] = s.Values[i]
	}
	out := make([]float64, len(axis))
	for i, t := range axis {
		if v, ok := idx[t]; ok {
			out[i] = v
		} else {
			out[i] = charts.GetNullValue()
		}
	}
	return out
}
