package svgchart

type scheme struct {
	bg     string
	canvas string
	text   string
	axis   string
	grid   string
	vline  string
	lines  []string
}

// darkScheme is a Catppuccin Mocha-inspired dark palette.
var darkScheme = scheme{
	bg:     "#1e1e2e",
	canvas: "#181825",
	text:   "#cdd6f4",
	axis:   "#6c7086",
	grid:   "#313244",
	vline:  "rgba(243,139,168,0.55)",
	lines: []string{
		"#7dc4e4", "#a6e3a1", "#fab387", "#cba6f7",
		"#f38ba8", "#89dceb", "#f9e2af", "#74c7ec",
	},
}

// lightScheme is a Catppuccin Latte-inspired light palette.
var lightScheme = scheme{
	bg:     "#eff1f5",
	canvas: "#e6e9ef",
	text:   "#4c4f69",
	axis:   "#8c8fa1",
	grid:   "#bcc0cc",
	vline:  "rgba(210,15,57,0.55)",
	lines: []string{
		"#1e66f5", "#40a02b", "#df8e1d", "#8839ef",
		"#d20f39", "#04a5e5", "#209fb5", "#e64553",
	},
}
