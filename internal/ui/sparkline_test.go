package ui

import "testing"

func TestSparkline_Empty(t *testing.T) {
	if got := Sparkline(nil); got != "" {
		t.Errorf("Sparkline(nil) = %q, want empty", got)
	}
	if got := Sparkline([]float64{}); got != "" {
		t.Errorf("Sparkline([]) = %q, want empty", got)
	}
}

func TestSparkline_AllZero(t *testing.T) {
	got := Sparkline([]float64{0, 0, 0})
	want := "▁▁▁"
	if got != want {
		t.Errorf("Sparkline(all zeros) = %q, want %q", got, want)
	}
}

func TestSparkline_AllMax(t *testing.T) {
	got := Sparkline([]float64{100, 100, 100})
	want := "███"
	if got != want {
		t.Errorf("Sparkline(all 100) = %q, want %q", got, want)
	}
}

func TestSparkline_Mixed(t *testing.T) {
	got := Sparkline([]float64{0, 50, 100})
	if len([]rune(got)) != 3 {
		t.Errorf("expected 3 runes, got %d", len([]rune(got)))
	}
	runes := []rune(got)
	// 0% -> ▁, 50% -> middle block, 100% -> █
	if runes[0] != '▁' {
		t.Errorf("0%% should map to ▁, got %c", runes[0])
	}
	if runes[2] != '█' {
		t.Errorf("100%% should map to █, got %c", runes[2])
	}
}

func TestSparkline_NegativeAndOverflow(t *testing.T) {
	got := Sparkline([]float64{-10, 150})
	runes := []rune(got)
	if runes[0] != '▁' {
		t.Errorf("negative should clamp to ▁, got %c", runes[0])
	}
	if runes[1] != '█' {
		t.Errorf("overflow should clamp to █, got %c", runes[1])
	}
}
