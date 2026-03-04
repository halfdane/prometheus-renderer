package svgchart

import (
	"fmt"
	"math"
	"time"
)

type xTick struct {
	T     time.Time
	Label string
}

type yTick struct {
	Value float64
	Label string
}

// xTicks generates evenly spaced time ticks for the given range.
func xTicks(start, end time.Time, rangeSeconds int) []xTick {
	var interval time.Duration
	var format string

	switch {
	case rangeSeconds <= 2*3600:
		interval = 15 * time.Minute
		format = "15:04"
	case rangeSeconds <= 12*3600:
		interval = time.Hour
		format = "15:04"
	case rangeSeconds <= 48*3600:
		interval = 6 * time.Hour
		format = "15:04"
	case rangeSeconds <= 14*86400:
		interval = 24 * time.Hour
		format = "01-02"
	default:
		interval = 7 * 24 * time.Hour
		format = "01-02"
	}

	// align first tick to a clean boundary
	first := alignUp(start, interval)
	var ticks []xTick
	for t := first; !t.After(end); t = t.Add(interval) {
		ticks = append(ticks, xTick{T: t, Label: t.Format(format)})
	}
	return ticks
}

// alignUp rounds t up to the next multiple of d (UTC-aligned).
func alignUp(t time.Time, d time.Duration) time.Time {
	unix := t.Unix()
	secs := int64(d.Seconds())
	rem := unix % secs
	if rem == 0 {
		return t
	}
	return time.Unix(unix-rem+secs, 0).UTC()
}

// niceStep rounds rawStep up to 1, 2, or 5 × 10^n.
func niceStep(rawStep float64) float64 {
	if rawStep <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(rawStep))
	mag := math.Pow(10, exp)
	norm := rawStep / mag
	switch {
	case norm <= 1:
		return mag
	case norm <= 2:
		return 2 * mag
	case norm <= 5:
		return 5 * mag
	default:
		return 10 * mag
	}
}

// yTicks computes nice tick values and their labels for the data range [dataMin, dataMax].
// It returns the ticks, and the extended niceMin/niceMax that encompass the data.
func yTicks(dataMin, dataMax float64) (ticks []yTick, niceMin, niceMax float64) {
	const targetTicks = 5

	span := dataMax - dataMin
	if span == 0 {
		span = 1
	}
	step := niceStep(span / targetTicks)
	niceMin = math.Floor(dataMin/step) * step
	niceMax = math.Ceil(dataMax/step) * step

	for v := niceMin; v <= niceMax+step*1e-9; v += step {
		rounded := math.Round(v/step) * step
		ticks = append(ticks, yTick{Value: rounded, Label: fmtY(rounded)})
	}
	return
}

// fmtY formats a y-axis value with SI suffixes for large/small magnitudes.
func fmtY(v float64) string {
	abs := math.Abs(v)
	switch {
	case abs == 0:
		return "0"
	case abs >= 1e9:
		return fmt.Sprintf("%.3gG", v/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%.3gM", v/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("%.3gk", v/1e3)
	case abs >= 0.01:
		return fmt.Sprintf("%.4g", v)
	default:
		return fmt.Sprintf("%.3g", v)
	}
}
