package svgchart

import (
	"math"
	"strings"
	"testing"
	"time"
)

// ---- xmlEsc -----------------------------------------------------------------

func TestXMLEsc(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"a & b", "a &amp; b"},
		{"<tag>", "&lt;tag&gt;"},
		{"a<b>c&d", "a&lt;b&gt;c&amp;d"},
		{`say "hi"`, `say "hi"`},   // quotes should NOT be escaped in text content
		{"", ""},
	}
	for _, c := range cases {
		got := xmlEsc(c.in)
		if got != c.want {
			t.Errorf("xmlEsc(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- seriesRange ------------------------------------------------------------

func TestSeriesRange(t *testing.T) {
	ts := []time.Time{time.Now()}

	t.Run("normal", func(t *testing.T) {
		s := []Series{{Timestamps: ts, Values: []float64{10, 50, 30}}}
		min, max := seriesRange(s)
		if min != 10 || max != 50 {
			t.Errorf("got (%v, %v), want (10, 50)", min, max)
		}
	})

	t.Run("multiSeries", func(t *testing.T) {
		s := []Series{
			{Timestamps: ts, Values: []float64{5, 20}},
			{Timestamps: ts, Values: []float64{-3, 100}},
		}
		min, max := seriesRange(s)
		if min != -3 || max != 100 {
			t.Errorf("got (%v, %v), want (-3, 100)", min, max)
		}
	})

	t.Run("allNaN", func(t *testing.T) {
		s := []Series{{Timestamps: ts, Values: []float64{math.NaN(), math.Inf(1)}}}
		min, max := seriesRange(s)
		// should fall back to default 0,1
		if min != 0 || max != 1 {
			t.Errorf("allNaN: got (%v, %v), want (0, 1)", min, max)
		}
	})

	t.Run("empty", func(t *testing.T) {
		min, max := seriesRange(nil)
		if min != 0 || max != 1 {
			t.Errorf("empty: got (%v, %v), want (0, 1)", min, max)
		}
	})

	t.Run("singleValue", func(t *testing.T) {
		s := []Series{{Timestamps: ts, Values: []float64{42}}}
		min, max := seriesRange(s)
		// min == max is normalised to ±1 around the value
		if min >= max {
			t.Errorf("singleValue: min %v >= max %v", min, max)
		}
	})
}

// ---- buildPath --------------------------------------------------------------

func TestBuildPath(t *testing.T) {
	now := time.Now()
	mkTS := func(n int) []time.Time {
		ts := make([]time.Time, n)
		for i := range ts {
			ts[i] = now.Add(time.Duration(i) * time.Minute)
		}
		return ts
	}

	toXY := func(t time.Time) float64 { return float64(t.Unix()) }
	identY := func(v float64) float64 { return v }

	t.Run("empty", func(t *testing.T) {
		s := Series{}
		if buildPath(s, toXY, identY) != "" {
			t.Error("expected empty string for empty series")
		}
	})

	t.Run("singlePoint", func(t *testing.T) {
		s := Series{Timestamps: mkTS(1), Values: []float64{5}}
		d := buildPath(s, toXY, identY)
		if !strings.HasPrefix(d, "M ") {
			t.Errorf("single point should start with M, got %q", d)
		}
		if strings.Contains(d, " L ") {
			t.Error("single point should not contain L command")
		}
	})

	t.Run("continuousRun", func(t *testing.T) {
		ts := mkTS(5)
		s := Series{Timestamps: ts, Values: []float64{1, 2, 3, 4, 5}}
		d := buildPath(s, toXY, identY)
		// one M, four L commands
		if strings.Count(d, "M ") != 1 {
			t.Errorf("expected 1 M, got path: %s", d)
		}
		if strings.Count(d, " L ") != 4 {
			t.Errorf("expected 4 L commands, got path: %s", d)
		}
	})

	t.Run("gapOnNaN", func(t *testing.T) {
		ts := mkTS(6)
		s := Series{Timestamps: ts, Values: []float64{1, 2, math.NaN(), math.NaN(), 5, 6}}
		d := buildPath(s, toXY, identY)
		// two separate runs → two M commands
		if strings.Count(d, "M ") != 2 {
			t.Errorf("expected 2 M commands for gap, got path: %s", d)
		}
	})

	t.Run("gapOnInf", func(t *testing.T) {
		ts := mkTS(4)
		s := Series{Timestamps: ts, Values: []float64{1, math.Inf(1), math.Inf(-1), 4}}
		d := buildPath(s, toXY, identY)
		if strings.Count(d, "M ") != 2 {
			t.Errorf("expected 2 M commands for Inf gap, got path: %s", d)
		}
	})

	t.Run("allNaN", func(t *testing.T) {
		ts := mkTS(3)
		s := Series{Timestamps: ts, Values: []float64{math.NaN(), math.NaN(), math.NaN()}}
		if buildPath(s, toXY, identY) != "" {
			t.Error("expected empty path for all-NaN series")
		}
	})
}

// ---- Render integration -----------------------------------------------------

func makeTestFigure(light bool) Figure {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)
	ts := make([]time.Time, 60)
	vals := make([]float64, 60)
	for i := range ts {
		ts[i] = start.Add(time.Duration(i*2) * time.Minute)
		vals[i] = float64(i) * 1.5
	}
	return Figure{
		Width: 800, PanelHeight: 300,
		Light:        light,
		TimeStart:    start,
		TimeEnd:      now,
		RangeSeconds: 7200,
		VLines:       []VLine{{Time: start.Add(30 * time.Minute)}},
		Panels: []Panel{{
			Title:  "Test",
			Series: []Series{{Label: "metric", Timestamps: ts, Values: vals}},
		}},
	}
}

func TestRenderProducesSVG(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	if err := Render(fig, &sb); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := sb.String()

	// Must open and close as an SVG document.
	if !strings.HasPrefix(out, "<svg ") {
		t.Error("output should start with <svg")
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "</svg>") {
		t.Error("output should end with </svg>")
	}

	// Width attribute present; height is dynamic (depends on legend rows).
	if !strings.Contains(out, `width="800"`) {
		t.Error("missing width attribute")
	}
	if !strings.Contains(out, `height="`) {
		t.Error("missing height attribute")
	}
}

func TestRenderLightScheme(t *testing.T) {
	fig := makeTestFigure(true)
	var sb strings.Builder
	if err := Render(fig, &sb); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := sb.String()
	// Light background colour should appear (Catppuccin Latte).
	if !strings.Contains(out, "#eff1f5") {
		t.Error("light scheme background colour not found in output")
	}
}

func TestRenderDarkScheme(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	if err := Render(fig, &sb); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "#1e1e2e") {
		t.Error("dark scheme background colour not found in output")
	}
}

func TestRenderContainsClipPath(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	_ = Render(fig, &sb)
	if !strings.Contains(sb.String(), "<clipPath") {
		t.Error("expected a <clipPath in the output")
	}
}

func TestRenderVLinesPresent(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	_ = Render(fig, &sb)
	out := sb.String()
	// The vline colour from the dark scheme should appear.
	if !strings.Contains(out, "rgba(243,139,168,0.55)") {
		t.Error("vline colour not found; vlines may not be rendering")
	}
}

func TestRenderSeriesPath(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	_ = Render(fig, &sb)
	if !strings.Contains(sb.String(), `<path d="M`) {
		t.Error("expected at least one <path element for series data")
	}
}

func TestRenderTitle(t *testing.T) {
	fig := makeTestFigure(false)
	var sb strings.Builder
	_ = Render(fig, &sb)
	if !strings.Contains(sb.String(), "Test") {
		t.Error("panel title not found in SVG output")
	}
}

func TestRenderMultiplePanels(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	ts := []time.Time{start, start.Add(30 * time.Minute), now}
	vals := []float64{1, 2, 3}

	fig := Figure{
		Width: 800, PanelHeight: 200,
		TimeStart: start, TimeEnd: now, RangeSeconds: 3600,
		Panels: []Panel{
			{Title: "Panel A", Series: []Series{{Label: "a", Timestamps: ts, Values: vals}}},
			{Title: "Panel B", Series: []Series{{Label: "b", Timestamps: ts, Values: vals}}},
		},
	}

	var sb strings.Builder
	if err := Render(fig, &sb); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := sb.String()

	// Each panel gets its own clipPath.
	if strings.Count(out, "<clipPath") != 2 {
		t.Errorf("expected 2 clipPaths for 2 panels, got %d", strings.Count(out, "<clipPath"))
	}
	// Total height = 2*200 + 1*panelGap.
	// Total height = 2*(PanelHeight + 1 legend row) + 1*panelGap
	// = 2*(200+legendRowH) + panelGap = 2*220 + 6 = 446
	wantH := 2*(200+legendRowH) + panelGap
	if !strings.Contains(out, `height="`+itoa(wantH)+`"`) {
		t.Errorf("expected SVG height %d in output", wantH)
	}
	// Both panel titles present.
	if !strings.Contains(out, "Panel A") || !strings.Contains(out, "Panel B") {
		t.Error("one or both panel titles not found in output")
	}
}

func TestRenderEmptyPanelsError(t *testing.T) {
	fig := Figure{Width: 800, PanelHeight: 300}
	var sb strings.Builder
	if err := Render(fig, &sb); err == nil {
		t.Error("expected error for figure with no panels")
	}
}

// ---- legendLayout ----------------------------------------------------------

func TestLegendLayoutNilWhenNoLabels(t *testing.T) {
	series := []Series{
		{Label: "", Values: []float64{1}},
		{Label: "", Values: []float64{2}},
	}
	if legendLayout(series, 800) != nil {
		t.Error("expected nil legend when all labels are empty")
	}
}

func TestLegendLayoutSingleRow(t *testing.T) {
	// Three short labels should all fit on one wide row.
	series := []Series{
		{Label: "cpu"},
		{Label: "mem"},
		{Label: "net"},
	}
	rows := legendLayout(series, 800)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 3 {
		t.Errorf("expected 3 items in row, got %d", len(rows[0]))
	}
}

func TestLegendLayoutWraps(t *testing.T) {
	// Each chip is roughly dotR*2+4 + chars*legendCharW + gap.
	// With plotW=100, force wrapping by using a label long enough.
	series := []Series{
		{Label: "series_one_long"},
		{Label: "series_two_long"},
	}
	rows := legendLayout(series, 100)
	if len(rows) < 2 {
		t.Errorf("expected wrapping into >=2 rows with plotW=100, got %d row(s)", len(rows))
	}
}

func TestLegendLayoutIndexOrdering(t *testing.T) {
	// Indices in the returned rows should cover all series, in order.
	series := []Series{
		{Label: "a"}, {Label: "b"}, {Label: "c"}, {Label: "d"},
	}
	rows := legendLayout(series, 800)
	var all []int
	for _, row := range rows {
		all = append(all, row...)
	}
	for i, idx := range all {
		if idx != i {
			t.Errorf("expected index %d at position %d, got %d", i, i, idx)
		}
	}
}

func TestLegendLayoutRendered(t *testing.T) {
	// Labels should appear in SVG output when series are labelled.
	fig := makeTestFigure(false)
	var sb strings.Builder
	_ = Render(fig, &sb)
	out := sb.String()
	// The test figure has one series labelled "metric".
	if !strings.Contains(out, "metric") {
		t.Error("series label not found in SVG output")
	}
	// A circle element should be present for the legend dot.
	if !strings.Contains(out, "<circle ") {
		t.Error("expected <circle element for legend dot")
	}
}

func TestLegendLayoutNoLabelNoCircle(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	ts := []time.Time{start, now}
	fig := Figure{
		Width: 800, PanelHeight: 200,
		TimeStart: start, TimeEnd: now, RangeSeconds: 3600,
		Panels: []Panel{{
			// No label → legend should be omitted.
			Series: []Series{{Timestamps: ts, Values: []float64{1, 2}}},
		}},
	}
	var sb strings.Builder
	_ = Render(fig, &sb)
	if strings.Contains(sb.String(), "<circle ") {
		t.Error("no legend circle expected when series has no label")
	}
}

// itoa is a tiny helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
