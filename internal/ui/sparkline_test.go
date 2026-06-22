package ui

import "testing"

func TestSparklineEmpty(t *testing.T) {
	if got := Sparkline(nil, 8); got != "n/a" {
		t.Fatalf("Sparkline empty = %q, want n/a", got)
	}
}

func TestSparklineScalesValues(t *testing.T) {
	got := Sparkline([]float64{0, 25, 50, 75, 100}, 5)
	want := "▁▂▄▆█"
	if got != want {
		t.Fatalf("Sparkline = %q, want %q", got, want)
	}
}

func TestSparklineUsesRightmostValues(t *testing.T) {
	got := Sparkline([]float64{1, 2, 3, 4}, 2)
	want := "▁█"
	if got != want {
		t.Fatalf("Sparkline = %q, want %q", got, want)
	}
}
