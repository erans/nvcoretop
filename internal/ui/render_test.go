package ui

import (
	"errors"
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

func TestRenderTensorWallNoColorMultipleGPUs(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithTensorActivity()))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	for _, want := range []string{
		"Tensor/DRAM Activity Wall",
		"GPU 0",
		"GPU 1",
		"GPU 2",
		"Tensor Pipe 92%",
		"DRAM 71%",
		"SM 84%",
		"FP32 33%",
		"Tensor Pipe unavailable",
		"DRAM 15%",
		"source NVML+DCGM",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("tensor wall missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("no-color tensor wall contains ANSI escapes: %q", view)
	}
}

func TestRenderTensorWallEmptySnapshot(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 20})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	for _, want := range []string{"Tensor/DRAM Activity Wall", "waiting for GPU samples"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty tensor wall missing %q in:\n%s", want, view)
		}
	}
}

func TestRenderTensorWallLineWidthBoundedNoColor(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithTensorActivity()
	for i := range snapshot.Devices {
		snapshot.Devices[i].Name = "NVIDIA H100 SXM5 80GB Very Long Engineering Sample Name"
	}
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 72, Height: 24})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) > model.height {
		t.Fatalf("tensor wall line count = %d, want <= %d:\n%s", len(lines), model.height, view)
	}
	for _, line := range lines {
		if got := utf8.RuneCountInString(line); got > model.width {
			t.Fatalf("tensor wall line length = %d, want <= %d:\n%s", got, model.width, view)
		}
	}
}

func TestRenderTensorWallFooterHelpWidthBoundedNoColor(t *testing.T) {
	model := NewModel(Options{NoColor: true, Interval: "250 milliseconds"})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithTensorActivity()))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 36, Height: 18})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		if got := utf8.RuneCountInString(line); got > model.width {
			t.Fatalf("tensor wall line length = %d, want <= %d:\n%s", got, model.width, view)
		}
	}
}

func TestRenderTensorWallLineBudgetPreservesFooterHelpAndOverflow(t *testing.T) {
	model := NewModel(Options{NoColor: true, Interval: "500ms"})
	snapshot := snapshotWithDevices(10)
	snapshot.Source = gpu.SourceNVMLDCGM
	for i := range snapshot.Devices {
		snapshot.Devices[i].Name = "H100-SXM5"
		snapshot.Devices[i].SMActivePct = gpu.Some(float64(80 - i))
		snapshot.Devices[i].TensorActivePct = gpu.Some(float64(90 - i))
		snapshot.Devices[i].MemPipeActivePct = gpu.Some(float64(70 - i))
		snapshot.Devices[i].FP32ActivePct = gpu.Some(float64(30 - i))
	}
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 78, Height: 12})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model, _ = updateModel(model, ErrMsg{Err: errors.New("poll failed")})

	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) > model.height {
		t.Fatalf("tensor wall line count = %d, want <= %d:\n%s", len(lines), model.height, view)
	}
	if lines[0] != "error: poll failed" {
		t.Fatalf("first line = %q, want error line in:\n%s", lines[0], view)
	}
	if !strings.Contains(view, "... 9 more GPU(s)") {
		t.Fatalf("tensor wall missing overflow line in:\n%s", view)
	}
	if !strings.Contains(lines[len(lines)-2], "interval 500ms") || !strings.Contains(lines[len(lines)-2], "source NVML+DCGM") {
		t.Fatalf("footer not preserved before help in:\n%s", view)
	}
	if !strings.Contains(lines[len(lines)-1], "keys:") {
		t.Fatalf("help not preserved as last line in:\n%s", view)
	}
}

func TestRenderTensorWallExactFitShowsGPUBlockBeforeOverflow(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithTensorActivity()
	snapshot.Devices = snapshot.Devices[:2]
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 8})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) > model.height {
		t.Fatalf("tensor wall line count = %d, want <= %d:\n%s", len(lines), model.height, view)
	}
	for _, want := range []string{"Tensor/DRAM Activity Wall", "GPU 0", "Tensor Pipe 92%", "DRAM 71%", "source NVML+DCGM"} {
		if !strings.Contains(view, want) {
			t.Fatalf("exact-fit tensor wall missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "... 2 more GPU(s)") {
		t.Fatalf("exact-fit tensor wall replaced fitting GPU block with overflow:\n%s", view)
	}
}

func TestRenderTensorWallHelpLabel(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithTensorActivity()))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 20})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	view := model.View()
	if !strings.Contains(view, "t toggle wall | o overview") {
		t.Fatalf("tensor wall help missing toggle label in:\n%s", view)
	}
	if strings.Contains(view, "t overview | o overview") {
		t.Fatalf("tensor wall help contains ambiguous overview label in:\n%s", view)
	}
}

func TestTensorGPUBlockClampsDisplayedTensorAndDRAMPercentages(t *testing.T) {
	device := gpu.DeviceSample{
		Index:            7,
		Name:             "H100-SXM5",
		TensorActivePct:  gpu.Some(125.0),
		MemPipeActivePct: gpu.Some(-12.0),
	}

	block := strings.Join(renderTensorGPUBlock(device, gpu.SourceNVMLDCGM, 80, 8, 1), "\n")
	for _, want := range []string{"Tensor Pipe 100%", "DRAM 0%", "████████", "░░░░░░░░"} {
		if !strings.Contains(block, want) {
			t.Fatalf("tensor block missing %q in:\n%s", want, block)
		}
	}
	for _, unwanted := range []string{"125%", "-12%", "MemPipe"} {
		if strings.Contains(block, unwanted) {
			t.Fatalf("tensor block contains %q in:\n%s", unwanted, block)
		}
	}
}

func TestTensorGPUBlockRendersDRAMUnavailableIndependently(t *testing.T) {
	device := snapshotWithTensorActivity().Devices[0]
	device.MemPipeActivePct = gpu.Optional[float64]{}

	block := strings.Join(renderTensorGPUBlock(device, gpu.SourceNVMLDCGM, 100, 12, 1), "\n")
	for _, want := range []string{"Tensor Pipe 92%", "DRAM unavailable (DCGM field missing)"} {
		if !strings.Contains(block, want) {
			t.Fatalf("tensor block missing %q in:\n%s", want, block)
		}
	}
	if strings.Contains(block, "MemPipe") {
		t.Fatalf("tensor block contains visible MemPipe label in:\n%s", block)
	}
}

func TestTensorHeatmapRowsClampOutOfRangeValues(t *testing.T) {
	if got := strings.Join(tensorHeatmapRows(gpu.Some(125.0), 5, 2), "\n"); got != "█████\n█████" {
		t.Fatalf("high clamp heatmap = %q, want full rows", got)
	}
	if got := strings.Join(tensorHeatmapRows(gpu.Some(-5.0), 5, 1), "\n"); got != "░░░░░" {
		t.Fatalf("low clamp heatmap = %q, want empty row", got)
	}
}

func TestRenderTensorWallUsesSortOrder(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithTensorActivity()
	snapshot.Devices[0].GPUUtil = gpu.Some(uint32(10))
	snapshot.Devices[1].GPUUtil = gpu.Some(uint32(90))
	snapshot.Devices[2].GPUUtil = gpu.Some(uint32(50))
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	first := strings.Index(view, "GPU 1")
	second := strings.Index(view, "GPU 2")
	third := strings.Index(view, "GPU 0")
	if first == -1 || second == -1 || third == -1 || !(first < second && second < third) {
		t.Fatalf("tensor wall GPU order not sorted by util in:\n%s", view)
	}
}

func snapshotWithTensorActivity() gpu.Snapshot {
	snapshot := snapshotWithDevices(3)
	snapshot.Source = gpu.SourceNVMLDCGM

	snapshot.Devices[0].Name = "H100-SXM5"
	snapshot.Devices[0].MemUsed = 48 * 1024 * 1024 * 1024
	snapshot.Devices[0].MemTotal = 80 * 1024 * 1024 * 1024
	snapshot.Devices[0].GPUUtil = gpu.Some(uint32(89))
	snapshot.Devices[0].MemUtil = gpu.Some(uint32(61))
	snapshot.Devices[0].TempC = gpu.Some(uint32(66))
	snapshot.Devices[0].SMActivePct = gpu.Some(84.0)
	snapshot.Devices[0].TensorActivePct = gpu.Some(92.0)
	snapshot.Devices[0].MemPipeActivePct = gpu.Some(71.0)
	snapshot.Devices[0].FP32ActivePct = gpu.Some(33.0)

	snapshot.Devices[1].Name = "H100-SXM5"
	snapshot.Devices[1].MemUsed = 32 * 1024 * 1024 * 1024
	snapshot.Devices[1].MemTotal = 80 * 1024 * 1024 * 1024
	snapshot.Devices[1].GPUUtil = gpu.Some(uint32(62))
	snapshot.Devices[1].MemUtil = gpu.Some(uint32(44))
	snapshot.Devices[1].TempC = gpu.Some(uint32(58))
	snapshot.Devices[1].SMActivePct = gpu.Some(68.0)
	snapshot.Devices[1].TensorActivePct = gpu.Some(63.0)
	snapshot.Devices[1].MemPipeActivePct = gpu.Some(51.0)
	snapshot.Devices[1].FP32ActivePct = gpu.Some(21.0)

	snapshot.Devices[2].Name = "H100-SXM5"
	snapshot.Devices[2].MemUsed = 12 * 1024 * 1024 * 1024
	snapshot.Devices[2].MemTotal = 80 * 1024 * 1024 * 1024
	snapshot.Devices[2].GPUUtil = gpu.Some(uint32(18))
	snapshot.Devices[2].MemUtil = gpu.Some(uint32(20))
	snapshot.Devices[2].TempC = gpu.Some(uint32(52))
	snapshot.Devices[2].SMActivePct = gpu.Some(19.0)
	snapshot.Devices[2].MemPipeActivePct = gpu.Some(15.0)
	snapshot.Devices[2].FP32ActivePct = gpu.Some(8.0)

	return snapshot
}
