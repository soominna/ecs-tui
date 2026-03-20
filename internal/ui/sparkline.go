package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a slice of 0–100 percentage values as a Unicode sparkline.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, v := range values {
		idx := int(v / 100.0 * float64(len(sparkBlocks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		sb.WriteRune(sparkBlocks[idx])
	}
	return sb.String()
}

// SparklineFit renders a sparkline that fits within maxWidth display cells,
// leaving room for the value suffix. Uses the most recent data points that fit.
// Handles CJK terminals where block characters may be 2 cells wide.
func SparklineFit(values []float64, maxWidth int, suffix string) string {
	if len(values) == 0 || maxWidth <= 0 {
		return suffix
	}
	suffixW := lipgloss.Width(" " + suffix)
	available := maxWidth - suffixW
	if available <= 0 {
		return suffix
	}
	// Try fitting most recent data points, reducing count until it fits
	for n := len(values); n > 0; n-- {
		spark := Sparkline(values[len(values)-n:])
		if lipgloss.Width(spark) <= available {
			return spark + " " + suffix
		}
	}
	return suffix
}
