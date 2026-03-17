package ui

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Theme names.
const (
	ThemeMocha = "mocha"
	ThemeLatte = "latte"
)

// palette holds Catppuccin color values for a single theme variant.
type palette struct {
	Crust     lipgloss.Color
	Mantle    lipgloss.Color
	Base      lipgloss.Color
	Surface0  lipgloss.Color
	Surface1  lipgloss.Color
	Surface2  lipgloss.Color
	Overlay0  lipgloss.Color
	Overlay1  lipgloss.Color
	Subtext0  lipgloss.Color
	Subtext1  lipgloss.Color
	Text      lipgloss.Color
	Blue      lipgloss.Color
	Lavender  lipgloss.Color
	Sapphire  lipgloss.Color
	Green     lipgloss.Color
	Teal      lipgloss.Color
	Red       lipgloss.Color
	Maroon    lipgloss.Color
	Peach     lipgloss.Color
	Yellow    lipgloss.Color
	Mauve     lipgloss.Color
	Rosewater lipgloss.Color
}

var mochaTheme = palette{
	Crust:     lipgloss.Color("#11111b"),
	Mantle:    lipgloss.Color("#181825"),
	Base:      lipgloss.Color("#1e1e2e"),
	Surface0:  lipgloss.Color("#313244"),
	Surface1:  lipgloss.Color("#45475a"),
	Surface2:  lipgloss.Color("#585b70"),
	Overlay0:  lipgloss.Color("#6c7086"),
	Overlay1:  lipgloss.Color("#7f849c"),
	Subtext0:  lipgloss.Color("#a6adc8"),
	Subtext1:  lipgloss.Color("#bac2de"),
	Text:      lipgloss.Color("#cdd6f4"),
	Blue:      lipgloss.Color("#89b4fa"),
	Lavender:  lipgloss.Color("#b4befe"),
	Sapphire:  lipgloss.Color("#74c7ec"),
	Green:     lipgloss.Color("#a6e3a1"),
	Teal:      lipgloss.Color("#94e2d5"),
	Red:       lipgloss.Color("#f38ba8"),
	Maroon:    lipgloss.Color("#eba0ac"),
	Peach:     lipgloss.Color("#fab387"),
	Yellow:    lipgloss.Color("#f9e2af"),
	Mauve:     lipgloss.Color("#cba6f7"),
	Rosewater: lipgloss.Color("#f5e0dc"),
}

var latteTheme = palette{
	Crust:     lipgloss.Color("#dce0e8"),
	Mantle:    lipgloss.Color("#e6e9ef"),
	Base:      lipgloss.Color("#eff1f5"),
	Surface0:  lipgloss.Color("#ccd0da"),
	Surface1:  lipgloss.Color("#bcc0cc"),
	Surface2:  lipgloss.Color("#acb0be"),
	Overlay0:  lipgloss.Color("#9ca0b0"),
	Overlay1:  lipgloss.Color("#8c8fa1"),
	Subtext0:  lipgloss.Color("#6c6f85"),
	Subtext1:  lipgloss.Color("#5c5f77"),
	Text:      lipgloss.Color("#4c4f69"),
	Blue:      lipgloss.Color("#1e66f5"),
	Lavender:  lipgloss.Color("#7287fd"),
	Sapphire:  lipgloss.Color("#209fb5"),
	Green:     lipgloss.Color("#40a02b"),
	Teal:      lipgloss.Color("#179299"),
	Red:       lipgloss.Color("#d20f39"),
	Maroon:    lipgloss.Color("#e64553"),
	Peach:     lipgloss.Color("#fe640b"),
	Yellow:    lipgloss.Color("#df8e1d"),
	Mauve:     lipgloss.Color("#8839ef"),
	Rosewater: lipgloss.Color("#dc8a78"),
}

var (
	themeMu      sync.RWMutex
	currentTheme = ThemeMocha
)

// CurrentThemeName returns the active theme name.
func CurrentThemeName() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return currentTheme
}

// ToggleTheme switches between Mocha (dark) and Latte (light).
func ToggleTheme() {
	themeMu.Lock()
	current := currentTheme
	themeMu.Unlock()
	if current == ThemeMocha {
		ApplyTheme(ThemeLatte)
	} else {
		ApplyTheme(ThemeMocha)
	}
}

// ApplyTheme sets all color and style variables to the given theme.
func ApplyTheme(name string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	var p palette
	switch name {
	case ThemeLatte:
		p = latteTheme
		currentTheme = ThemeLatte
	default:
		p = mochaTheme
		currentTheme = ThemeMocha
	}

	// ── styles.go colors ──
	colorCrust = p.Crust
	colorMantle = p.Mantle
	colorBase = p.Base
	colorSurface0 = p.Surface0
	colorSurface1 = p.Surface1
	colorSurface2 = p.Surface2
	colorOverlay0 = p.Overlay0
	colorOverlay1 = p.Overlay1
	colorSubtext0 = p.Subtext0
	colorSubtext1 = p.Subtext1
	colorText = p.Text
	colorBlue = p.Blue
	colorLavender = p.Lavender
	colorSapphire = p.Sapphire
	colorGreen = p.Green
	colorTeal = p.Teal
	colorRed = p.Red
	colorMaroon = p.Maroon
	colorPeach = p.Peach
	colorYellow = p.Yellow
	colorMauve = p.Mauve
	colorRosewater = p.Rosewater

	// semantic aliases
	colorPrimary = colorBlue
	colorAccent = colorTeal
	colorError = colorRed
	colorWarning = colorYellow
	colorMuted = colorOverlay0
	colorWhite = colorText
	colorDimText = colorSubtext0

	// ── Header/footer bar backgrounds ──
	headerBg := colorSurface0
	if name == ThemeLatte {
		headerBg = lipgloss.Color("#ffffff")
	}

	// ── styles.go styles ──
	headerStyle = lipgloss.NewStyle().
		Background(headerBg).
		Foreground(colorText).
		Bold(true).
		Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
		Background(colorMantle).
		Foreground(colorSubtext1).
		Padding(0, 1)

	shortcutKeyStyle = lipgloss.NewStyle().
		Foreground(colorBlue).
		Bold(true)

	shortcutDescStyle = lipgloss.NewStyle().
		Foreground(colorSubtext0)

	errorStyle = lipgloss.NewStyle().
		Foreground(colorRed).
		Bold(true)

	statusStyle = lipgloss.NewStyle().
		Foreground(colorGreen)

	StatusRunning = lipgloss.NewStyle().Foreground(colorGreen)
	StatusStopped = lipgloss.NewStyle().Foreground(colorRed)
	StatusPending = lipgloss.NewStyle().Foreground(colorPeach)
	StatusActive = lipgloss.NewStyle().Foreground(colorGreen)

	titleStyle = lipgloss.NewStyle().
		Foreground(colorSubtext0).
		Bold(true)

	sectionTitleStyle = lipgloss.NewStyle().
		Foreground(colorBlue).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(colorSurface1).
		Padding(0, 0, 0, 1).
		MarginBottom(1)

	breadcrumbStyle = lipgloss.NewStyle().
		Foreground(colorOverlay1)

	breadcrumbActiveStyle = lipgloss.NewStyle().
		Foreground(colorBlue).
		Bold(true)

	breadcrumbSepStyle = lipgloss.NewStyle().
		Foreground(colorSurface2)

	loadingStyle = lipgloss.NewStyle().
		Foreground(colorOverlay1).
		Italic(true)

	// ── help.go styles ──
	helpTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorBlue).
		MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
		Foreground(colorTeal).
		Bold(true).
		Width(15)

	helpDescStyle = lipgloss.NewStyle().
		Foreground(colorSubtext1)

	helpBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSurface2).
		Padding(1, 2)

	// ── config.go styles ──
	configBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(0, 1)

	configLabelStyle = lipgloss.NewStyle().
		Foreground(colorSubtext0).
		Bold(true)

	configValueStyle = lipgloss.NewStyle().
		Foreground(colorText).
		Bold(true)

	stepLabelStyle = lipgloss.NewStyle().
		Background(colorBlue).
		Foreground(colorCrust).
		Bold(true).
		Padding(0, 1)

	stepDescStyle = lipgloss.NewStyle().
		Foreground(colorSubtext1).
		Padding(0, 1)
}

func init() {
	ApplyTheme(ThemeMocha)
}

// logoText is a compact 3-line ASCII logo.
const logoText = ` ___  ___  ___    _____  _   _  ___
| __||  _|/ __|  |_   _|| | | ||_ _|
| _| | (_ \__ \    | |  | |_| | | |
|___| \__||___/    |_|   \___/ |___|`
