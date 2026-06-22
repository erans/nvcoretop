package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"nvcoretop/internal/gpu"
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

func TestRenderFooterIncludesIntervalSortAndSource(t *testing.T) {
	model := NewModel(Options{NoColor: true, Interval: "250ms"})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(1)))
	model.width = 100
	model.height = 30

	view := model.View()
	for _, want := range []string{"interval 250ms", "sort index", "source NVML"} {
		if !strings.Contains(view, want) {
			t.Fatalf("footer missing %q in:\n%s", want, view)
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

func TestRenderDetailWithoutHistoryDoesNotPanic(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model.snapshot = snapshotWithDevices(1)
	model.detail = true
	model.width = 100
	model.height = 30

	view := model.View()
	for _, want := range []string{"Util   n/a", "Temp   n/a", "Power  n/a"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail without history missing %q in:\n%s", want, view)
		}
	}
}

func TestRenderOverviewDCGMCoresStaySingleLine(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithDevices(1)
	snapshot.Source = gpu.SourceNVMLDCGM
	snapshot.Devices[0].SMActivePct = gpu.Some(45.0)
	snapshot.Devices[0].TensorActivePct = gpu.Some(12.0)
	snapshot.Devices[0].MemPipeActivePct = gpu.Some(34.0)
	snapshot.Devices[0].FP32ActivePct = gpu.Some(56.0)
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model.width = 100
	model.height = 30

	view := model.View()
	for _, unwanted := range []string{"Tensor", "MemPipe", "FP32"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("overview contains multiline DCGM label %q in:\n%s", unwanted, view)
		}
	}
}

func TestRenderDegradedLongNameWidthBounded(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithDevices(1)
	snapshot.Devices[0].Name = "NVIDIA RTX 6000 Ada Generation Very Long Engineering Sample"
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model.width = 60
	model.height = 20

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		if got := utf8.RuneCountInString(line); got > model.width {
			t.Fatalf("degraded line length = %d, want <= %d:\n%s", got, model.width, view)
		}
	}
}

func TestRenderDegradedDetailLongNameWidthBounded(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithDevices(1)
	snapshot.Devices[0].Name = "NVIDIA RTX 6000 Ada Generation Very Long Engineering Sample"
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model.detail = true
	model.width = 60
	model.height = 20

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		if got := utf8.RuneCountInString(line); got > model.width {
			t.Fatalf("degraded detail line length = %d, want <= %d:\n%s", got, model.width, view)
		}
	}
}
