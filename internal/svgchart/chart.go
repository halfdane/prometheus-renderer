// Package svgchart renders Prometheus time-series data as SVG images using
// only the Go standard library — no external dependencies.
package svgchart

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

// Layout constants (pixels).
const (
	marginLeft    = 60   // space for y-axis labels
	marginRight   = 10   // small right padding
	marginTop     = 28   // space for panel title
	marginBottom  = 34   // space for x-axis labels
	panelGap      = 6    // vertical gap between stacked panels
	legendRowH    = 20   // height of one legend row
	legendDotR    = 4    // radius of the colour dot
	legendCharW   = 6.6  // approximate monospace character width at font-size 11
	legendItemGap = 16   // horizontal gap between consecutive legend items
)

// ---- Public types -----------------------------------------------------------

// Figure is the top-level rendering container.
type Figure struct {
	Width        int
	PanelHeight  int       // height of each individual panel in pixels
	Light        bool      // use light colour scheme
	Smooth       bool      // draw series as smooth curves (cardinal spline) instead of straight lines
	TimeStart    time.Time // left edge of the time axis
	TimeEnd      time.Time // right edge of the time axis
	RangeSeconds int       // total range in seconds (used for tick spacing)
	VLines       []VLine   // vertical event markers shown on every panel
	Panels       []Panel
}

// Panel is one chart within a Figure, sharing the same time axis.
type Panel struct {
	Title  string
	Series []Series // multiple series share the same y-axis scale
	VLines []VLine  // panel-local event markers (merged with Figure.VLines)
}

// Series holds a single time-series to be drawn as a line.
type Series struct {
	Label      string
	Timestamps []time.Time
	Values     []float64
}

// VLine is a vertical marker drawn at a specific time.
type VLine struct {
	Time time.Time
}

// ---- Entry point -----------------------------------------------------------

// Render writes an SVG document representing fig to w.
func Render(fig Figure, w io.Writer) error {
	sc := darkScheme
	if fig.Light {
		sc = lightScheme
	}

	n := len(fig.Panels)
	if n == 0 {
		return fmt.Errorf("svgchart: no panels")
	}

	plotW := fig.Width - marginLeft - marginRight

	// Pre-compute legend layout per panel so the total SVG height is known
	// before writing any output.
	allLegendRows := make([][][]int, n)
	panelTotalH := make([]int, n)
	for i, p := range fig.Panels {
		rows := legendLayout(p.Series, plotW)
		allLegendRows[i] = rows
		panelTotalH[i] = fig.PanelHeight + len(rows)*legendRowH
	}

	totalH := (n - 1) * panelGap
	for _, h := range panelTotalH {
		totalH += h
	}

	// Open SVG root.
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" style="background:%s;font-family:monospace,sans-serif">%s`,
		fig.Width, totalH, sc.bg, "\n")

	yOffset := 0
	for i, panel := range fig.Panels {
		// Merge figure-level and panel-level vlines.
		merged := make([]VLine, 0, len(fig.VLines)+len(panel.VLines))
		merged = append(merged, fig.VLines...)
		merged = append(merged, panel.VLines...)

		if err := renderPanel(w, panel, merged, allLegendRows[i], panelTotalH[i], fig, sc, yOffset, i); err != nil {
			return err
		}
		yOffset += panelTotalH[i] + panelGap
	}

	fmt.Fprintln(w, `</svg>`)
	return nil
}

// ---- Panel renderer --------------------------------------------------------

func renderPanel(w io.Writer, panel Panel, vlines []VLine, legendRows [][]int, totalH int, fig Figure, sc scheme, yOff, idx int) error {
	ph := fig.PanelHeight
	pw := fig.Width

	plotX := marginLeft
	plotY := marginTop
	plotW := pw - marginLeft - marginRight
	plotH := ph - marginTop - marginBottom

	clipID := fmt.Sprintf("clip%d", idx)

	// Canvas background covers plot area plus legend strip.
	fmt.Fprintf(w, `  <rect x="0" y="%d" width="%d" height="%d" fill="%s"/>%s`,
		yOff, pw, totalH, sc.canvas, "\n")

	// Clip path restricts series lines to the plot area.
	// Coordinates are in the group-local space (after the translate below).
	fmt.Fprintf(w, `  <defs><clipPath id="%s"><rect x="%d" y="%d" width="%d" height="%d"/></clipPath></defs>%s`,
		clipID, plotX, plotY, plotW, plotH, "\n")

	// Open panel group.
	fmt.Fprintf(w, `  <g transform="translate(0,%d)">%s`, yOff, "\n")

	// Title.
	if panel.Title != "" {
		fmt.Fprintf(w, `    <text x="%d" y="%d" fill="%s" font-size="13" text-anchor="middle">%s</text>%s`,
			plotX+plotW/2, marginTop-6, sc.text, xmlEsc(panel.Title), "\n")
	}

	// Compute y-axis range across all series.
	dataMin, dataMax := seriesRange(panel.Series)
	ticks, niceMin, niceMax := yTicks(dataMin, dataMax)

	ySpan := niceMax - niceMin
	if ySpan == 0 {
		ySpan = 1
	}
	tSpan := fig.TimeEnd.Sub(fig.TimeStart).Seconds()
	if tSpan <= 0 {
		tSpan = 1
	}

	// Helpers: data → pixel.
	toX := func(t time.Time) float64 {
		frac := t.Sub(fig.TimeStart).Seconds() / tSpan
		return float64(plotX) + frac*float64(plotW)
	}
	toY := func(v float64) float64 {
		frac := (v - niceMin) / ySpan
		return float64(plotY+plotH) - frac*float64(plotH)
	}

	// --- Y-axis grid lines and labels ---
	for _, tick := range ticks {
		py := int(math.Round(toY(tick.Value)))
		if py < plotY || py > plotY+plotH {
			continue
		}
		// Grid line.
		fmt.Fprintf(w, `    <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>%s`,
			plotX, py, plotX+plotW, py, sc.grid, "\n")
		// Label.
		fmt.Fprintf(w, `    <text x="%d" y="%d" fill="%s" font-size="11" text-anchor="end" dominant-baseline="middle">%s</text>%s`,
			plotX-4, py, sc.axis, xmlEsc(tick.Label), "\n")
	}

	// Y-axis border line.
	fmt.Fprintf(w, `    <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>%s`,
		plotX, plotY, plotX, plotY+plotH, sc.axis, "\n")

	// --- X-axis ticks and labels ---
	xticks := xTicks(fig.TimeStart, fig.TimeEnd, fig.RangeSeconds)
	for _, tick := range xticks {
		px := int(math.Round(toX(tick.T)))
		if px < plotX || px > plotX+plotW {
			continue
		}
		// Tick mark.
		fmt.Fprintf(w, `    <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>%s`,
			px, plotY+plotH, px, plotY+plotH+5, sc.axis, "\n")
		// Label.
		fmt.Fprintf(w, `    <text x="%d" y="%d" fill="%s" font-size="11" text-anchor="middle">%s</text>%s`,
			px, plotY+plotH+18, sc.axis, xmlEsc(tick.Label), "\n")
	}

	// X-axis border line.
	fmt.Fprintf(w, `    <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>%s`,
		plotX, plotY+plotH, plotX+plotW, plotY+plotH, sc.axis, "\n")

	// --- Vertical event lines (rendered inside clip) ---
	if len(vlines) > 0 {
		fmt.Fprintf(w, `    <g clip-path="url(#%s)">%s`, clipID, "\n")
		for _, vl := range vlines {
			px := math.Round(toX(vl.Time))
			fmt.Fprintf(w, `      <line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="%s" stroke-width="1.5"/>%s`,
				px, plotY, px, plotY+plotH, sc.vline, "\n")
		}
		fmt.Fprintln(w, `    </g>`)
	}

	// --- Series lines (rendered inside clip) ---
	fmt.Fprintf(w, `    <g clip-path="url(#%s)">%s`, clipID, "\n")
	for si, s := range panel.Series {
		color := sc.lines[si%len(sc.lines)]
		var d string
		if fig.Smooth {
			d = buildSmoothPath(s, toX, toY)
		} else {
			d = buildPath(s, toX, toY)
		}
		if d == "" {
			continue
		}
		fmt.Fprintf(w, `      <path d="%s" fill="none" stroke="%s" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>%s`,
			d, color, "\n")
	}
	fmt.Fprintln(w, `    </g>`)

	// --- Grafana-style legend strip (below x-axis labels) ---
	for rowIdx, row := range legendRows {
		rowY := ph + rowIdx*legendRowH + legendRowH/2
		x := float64(plotX)
		for _, si := range row {
			s := panel.Series[si]
			color := sc.lines[si%len(sc.lines)]
			fmt.Fprintf(w, `    <circle cx="%.1f" cy="%d" r="%d" fill="%s"/>%s`,
				x+legendDotR, rowY, legendDotR, color, "\n")
			fmt.Fprintf(w, `    <text x="%.1f" y="%d" fill="%s" font-size="11" dominant-baseline="middle">%s</text>%s`,
				x+float64(legendDotR*2+4), rowY, sc.text, xmlEsc(s.Label), "\n")
			x += float64(legendDotR*2+4) + float64(len(s.Label))*legendCharW + legendItemGap
		}
	}

	// Close panel group.
	fmt.Fprintln(w, `  </g>`)
	return nil
}

// ---- Helpers ----------------------------------------------------------------

// smoothTension controls how tightly the cardinal spline follows the data.
// 0 = straight lines; ~0.4 gives pleasing curves without excessive overshoot.
const smoothTension = 0.4

// buildSmoothPath converts a series to an SVG path using cubic bezier curves.
// Control points are derived via a cardinal spline so the curve passes exactly
// through every data point. NaN/Inf values produce visible gaps as with buildPath.
func buildSmoothPath(s Series, toX func(time.Time) float64, toY func(float64) float64) string {
	if len(s.Values) == 0 {
		return ""
	}

	type pt struct{ x, y float64 }

	// Split into runs of consecutive finite values.
	var runs [][]pt
	var run []pt
	for i, v := range s.Values {
		if i >= len(s.Timestamps) {
			break
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			if len(run) > 0 {
				runs = append(runs, run)
				run = nil
			}
			continue
		}
		run = append(run, pt{toX(s.Timestamps[i]), toY(v)})
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}

	var sb strings.Builder
	for ri, pts := range runs {
		n := len(pts)
		if n == 0 {
			continue
		}
		if ri > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "M %.2f %.2f", pts[0].x, pts[0].y)
		if n == 1 {
			continue
		}
		for i := 0; i < n-1; i++ {
			prev := pts[max(0, i-1)]
			curr := pts[i]
			next := pts[i+1]
			next2 := pts[min(n-1, i+2)]
			// Cardinal spline control points.
			cp1x := curr.x + (next.x-prev.x)*smoothTension
			cp1y := curr.y + (next.y-prev.y)*smoothTension
			cp2x := next.x - (next2.x-curr.x)*smoothTension
			cp2y := next.y - (next2.y-curr.y)*smoothTension
			fmt.Fprintf(&sb, " C %.2f,%.2f %.2f,%.2f %.2f,%.2f",
				cp1x, cp1y, cp2x, cp2y, next.x, next.y)
		}
	}
	return sb.String()
}

// buildPath converts a series to an SVG path d= attribute string.
// NaN/Inf values cause a gap (new M command) so the line is visually broken.
func buildPath(s Series, toX func(time.Time) float64, toY func(float64) float64) string {
	if len(s.Values) == 0 {
		return ""
	}

	var sb strings.Builder
	inRun := false

	for i, v := range s.Values {
		if i >= len(s.Timestamps) {
			break
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			inRun = false
			continue
		}
		x := toX(s.Timestamps[i])
		y := toY(v)
		if !inRun {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			fmt.Fprintf(&sb, "M %.2f %.2f", x, y)
			inRun = true
		} else {
			fmt.Fprintf(&sb, " L %.2f %.2f", x, y)
		}
	}
	return sb.String()
}

// seriesRange returns the overall min/max of all finite values across all series.
func seriesRange(series []Series) (float64, float64) {
	min, max := math.MaxFloat64, -math.MaxFloat64
	for _, s := range series {
		for _, v := range s.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}
	if min == math.MaxFloat64 {
		return 0, 1
	}
	if min == max {
		min -= 1
		max += 1
	}
	return min, max
}

// legendLayout computes wrapped legend rows for a panel's series.
// Returns nil if no series have labels (legend omitted entirely).
// Each returned []int is one row of series indices.
func legendLayout(series []Series, plotW int) [][]int {
	hasLabel := false
	for _, s := range series {
		if s.Label != "" {
			hasLabel = true
			break
		}
	}
	if !hasLabel {
		return nil
	}

	chipW := func(label string) float64 {
		return float64(legendDotR*2+4) + float64(len(label))*legendCharW + legendItemGap
	}

	var rows [][]int
	var row []int
	rowX := 0.0
	for i, s := range series {
		w := chipW(s.Label)
		if len(row) > 0 && rowX+w > float64(plotW) {
			rows = append(rows, row)
			row = nil
			rowX = 0
		}
		row = append(row, i)
		rowX += w
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows
}

// xmlEsc escapes the minimal set of characters required for SVG text content.
func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
