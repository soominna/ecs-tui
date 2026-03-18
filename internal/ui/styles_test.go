package ui

import "testing"

func TestCalcColumnWidths(t *testing.T) {
	t.Run("fixed columns only", func(t *testing.T) {
		cols := []responsiveColumn{
			{Title: "A", MinWidth: 10, Flex: 0},
			{Title: "B", MinWidth: 20, Flex: 0},
		}
		widths := calcColumnWidths(cols, 100)
		if widths[0] != 10 {
			t.Errorf("col A width = %d, want 10", widths[0])
		}
		if widths[1] != 20 {
			t.Errorf("col B width = %d, want 20", widths[1])
		}
	})

	t.Run("flex columns distribute extra space", func(t *testing.T) {
		cols := []responsiveColumn{
			{Title: "A", MinWidth: 10, Flex: 1},
			{Title: "B", MinWidth: 10, Flex: 1},
		}
		// totalWidth=100, available=100-2*2-1=95, fixedUsed=20, extra=75
		widths := calcColumnWidths(cols, 100)
		if widths[0] < 10 {
			t.Errorf("col A width = %d, should be >= 10", widths[0])
		}
		if widths[1] < 10 {
			t.Errorf("col B width = %d, should be >= 10", widths[1])
		}
		// Equal flex should give equal widths
		if widths[0] != widths[1] {
			t.Errorf("equal flex should give equal widths: %d vs %d", widths[0], widths[1])
		}
	})

	t.Run("unequal flex weights", func(t *testing.T) {
		cols := []responsiveColumn{
			{Title: "A", MinWidth: 10, Flex: 3},
			{Title: "B", MinWidth: 10, Flex: 1},
		}
		widths := calcColumnWidths(cols, 100)
		// Col A should get more space than col B
		if widths[0] <= widths[1] {
			t.Errorf("col A (flex=3) = %d should be > col B (flex=1) = %d", widths[0], widths[1])
		}
	})

	t.Run("very narrow terminal", func(t *testing.T) {
		cols := []responsiveColumn{
			{Title: "A", MinWidth: 10, Flex: 1},
			{Title: "B", MinWidth: 10, Flex: 1},
		}
		widths := calcColumnWidths(cols, 5)
		// Should not panic, widths should be at minimum
		if widths[0] != 10 {
			t.Errorf("col A width = %d, want 10 (minimum)", widths[0])
		}
	})
}
