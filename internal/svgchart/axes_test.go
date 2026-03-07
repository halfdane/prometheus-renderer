package svgchart

import (
	"math"
	"testing"
	"time"
)

// ---- niceStep ---------------------------------------------------------------

func TestNiceStep(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 1},
		{-5, 1},
		{1, 1},
		{1.1, 2},
		{2.1, 5},
		{5.1, 10},
		{9.9, 10},
		{10, 10},
		{15, 20},
		{33, 50},
		{55, 100},
		{99, 100},
		{100, 100},
		{101, 200},
		{0.03, 0.05},
		{0.007, 0.01},
	}

	for _, c := range cases {
		got := niceStep(c.in)
		if math.Abs(got-c.want)/math.Max(1, math.Abs(c.want)) > 1e-9 {
			t.Errorf("niceStep(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ---- fmtY -------------------------------------------------------------------

func TestFmtY(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{1234, "1.23k"},
		{-1234, "-1.23k"},
		{1_000_000, "1M"},
		{2_500_000, "2.5M"},
		{1_000_000_000, "1G"},
		{1_500_000_000, "1.5G"},
		{0.1, "0.1"},
		{0.001, "0.001"},
		{0.0001, "0.0001"},
		{0.00001, "1e-05"}, // %.3g uses scientific notation below 0.01
		{1.2345, "1.234"},
		{99.99, "99.99"},
	}

	for _, c := range cases {
		got := fmtY(c.in)
		if got != c.want {
			t.Errorf("fmtY(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- alignUp ----------------------------------------------------------------

func TestAlignUp(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// already aligned
	got := alignUp(base, 15*time.Minute)
	if !got.Equal(base) {
		t.Errorf("alignUp on boundary: got %v, want %v", got, base)
	}

	// 1 second past 15-min boundary → next 15-min mark
	t1 := base.Add(1 * time.Second)
	want := base.Add(15 * time.Minute)
	got = alignUp(t1, 15*time.Minute)
	if !got.Equal(want) {
		t.Errorf("alignUp(+1s, 15m) = %v, want %v", got, want)
	}

	// 14m59s past boundary → next mark
	t2 := base.Add(14*time.Minute + 59*time.Second)
	got = alignUp(t2, 15*time.Minute)
	if !got.Equal(want) {
		t.Errorf("alignUp(+14m59s, 15m) = %v, want %v", got, want)
	}

	// hourly boundary
	t3 := base.Add(30 * time.Minute)
	wantH := base.Add(time.Hour)
	got = alignUp(t3, time.Hour)
	if !got.Equal(wantH) {
		t.Errorf("alignUp(+30m, 1h) = %v, want %v", got, wantH)
	}
}

// ---- yTicks -----------------------------------------------------------------

func TestYTicksBounds(t *testing.T) {
	ticks, niceMin, niceMax := yTicks(10, 90)

	if niceMin > 10 {
		t.Errorf("niceMin %v should be <= dataMin 10", niceMin)
	}
	if niceMax < 90 {
		t.Errorf("niceMax %v should be >= dataMax 90", niceMax)
	}
	if len(ticks) < 2 {
		t.Errorf("expected at least 2 ticks, got %d", len(ticks))
	}
	// All tick values should be within [niceMin, niceMax].
	for _, tk := range ticks {
		if tk.Value < niceMin-1e-9 || tk.Value > niceMax+1e-9 {
			t.Errorf("tick %v outside [%v, %v]", tk.Value, niceMin, niceMax)
		}
	}
}

func TestYTicksFlatLine(t *testing.T) {
	// dataMin == dataMax → should not panic and should return a usable range.
	ticks, niceMin, niceMax := yTicks(42, 42)
	if niceMin >= niceMax {
		t.Errorf("niceMin %v should be < niceMax %v for flat line", niceMin, niceMax)
	}
	if len(ticks) < 2 {
		t.Errorf("expected at least 2 ticks, got %d", len(ticks))
	}
}

func TestYTicksLabelsNotEmpty(t *testing.T) {
	ticks, _, _ := yTicks(0, 1000)
	for _, tk := range ticks {
		if tk.Label == "" {
			t.Errorf("empty label for tick value %v", tk.Value)
		}
	}
}

// ---- xTicks -----------------------------------------------------------------

func TestXTicksIntervals(t *testing.T) {
	base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		rangeSeconds     int
		wantMinInterval  time.Duration
		wantMaxInterval  time.Duration
		wantLabelLen     int // all tick labels should have this length
	}{
		{1 * 3600, 15 * time.Minute, 15 * time.Minute, 5},       // ≤2h → 15min, "15:04"
		{6 * 3600, time.Hour, time.Hour, 5},                      // ≤12h → 1h
		{24 * 3600, 6 * time.Hour, 6 * time.Hour, 5},             // ≤48h → 6h
		{7 * 86400, 24 * time.Hour, 24 * time.Hour, 5},           // ≤14d → 1d, "01-02"
		{30 * 86400, 7 * 24 * time.Hour, 7 * 24 * time.Hour, 5}, // >14d → 7d
	}

	for _, c := range cases {
		end := base.Add(time.Duration(c.rangeSeconds) * time.Second)
		ticks := xTicks(base, end, c.rangeSeconds)
		if len(ticks) == 0 {
			t.Errorf("rangeSeconds=%d: no ticks returned", c.rangeSeconds)
			continue
		}
		// Check spacing between consecutive ticks.
		for i := 1; i < len(ticks); i++ {
			gap := ticks[i].T.Sub(ticks[i-1].T)
			if gap != c.wantMinInterval {
				t.Errorf("rangeSeconds=%d: gap[%d]=%v, want %v", c.rangeSeconds, i, gap, c.wantMinInterval)
			}
		}
		// Check label length (rough format check).
		for _, tk := range ticks {
			if len(tk.Label) != c.wantLabelLen {
				t.Errorf("rangeSeconds=%d: label %q has len %d, want %d", c.rangeSeconds, tk.Label, len(tk.Label), c.wantLabelLen)
			}
		}
	}
}

func TestXTicksAllWithinRange(t *testing.T) {
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	end := base.Add(24 * time.Hour)
	ticks := xTicks(base, end, 86400)

	for _, tk := range ticks {
		if tk.T.Before(base) || tk.T.After(end) {
			t.Errorf("tick %v is outside [%v, %v]", tk.T, base, end)
		}
	}
}
