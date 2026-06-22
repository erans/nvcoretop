package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderOverviewNoColor(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(2)))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.View()
	for _, want := range []string{"#", "NAME", "UTIL", "MEM", "TEMP", "PWR", "CORES", "RTX"} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("no-color view contains ANSI escapes: %q", view)
	}
}

func TestRenderDetail(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(1)))
	model.detail = true
	model.width = 100
	model.height = 30

	view := model.View()
	for _, want := range []string{"Detail GPU 0", "Util", "Temp", "Power", "Processes", "PCIe", "ECC"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail missing %q in:\n%s", want, view)
		}
	}
}

func TestRenderDegradedLayout(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(1)))
	model.width = 60
	model.height = 20

	view := model.View()
	if !strings.Contains(view, "GPU 0 RTX") || strings.Contains(view, "CORES") {
		t.Fatalf("degraded view unexpected:\n%s", view)
	}
}
