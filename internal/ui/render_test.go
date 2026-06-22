package ui

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestRenderOverviewUsesColorWhenEnabled(t *testing.T) {
	model := NewModel(Options{})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(2)))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.View()
	assertPlainLineColored(t, view, "#  NAME        UTIL")
	selected := lineByPlainContains(t, view, "> 0  RTX")
	assertColoredSegmentOccurrence(t, selected, ">", 1)
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

func TestRenderTensorWallUsesColorWhenEnabled(t *testing.T) {
	model := NewModel(Options{})
	snapshot := snapshotWithTensorActivity()
	snapshot.Devices[0].Name = "Tensor Pipe 92%"
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	view := model.View()
	assertPlainLineColored(t, view, "Tensor/DRAM Activity Wall")

	header := lineByPlainContains(t, view, "GPU 0 Tensor Pipe 92%  Tensor Pipe 92%  DRAM 71%")
	assertColoredSegmentOccurrence(t, header, "Tensor Pipe 92%", 2)
	assertColoredSegmentOccurrence(t, header, "DRAM 71%", 1)

	summary := lineByPlainEqual(t, view, "  Tensor Pipe 92%")
	assertColoredSegmentOccurrence(t, summary, "Tensor Pipe", 1)
	assertColoredSegmentOccurrence(t, summary, "92%", 1)

	heatmap := lineByPlainContains(t, view, "  █")
	assertColoredSegmentOccurrence(t, heatmap, "█", 1)

	assertPlainLineColored(t, view, "running | interval")
	assertPlainLineColored(t, view, "keys: t toggle wall")
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

func TestRenderTensorWallColoredLineWidthBounded(t *testing.T) {
	model := NewModel(Options{Interval: "very-long-refresh-interval-for-width-test"})
	snapshot := snapshotWithTensorActivity()
	for i := range snapshot.Devices {
		snapshot.Devices[i].Name = "Ｈ１００ ＳＸＭ５ ８０ＧＢ Very Long Engineering Sample Name"
	}
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 48, Height: 18})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	view := model.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("colored tensor wall missing ANSI escapes:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		plain := stripANSI(line)
		if got := lipgloss.Width(plain); got > model.width {
			t.Fatalf("colored tensor wall plain line display width = %d, want <= %d:\n%s", got, model.width, view)
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

func TestRenderTensorWallDropsSecondaryContextBeforePrimaryBlocks(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithTensorActivity()
	snapshot.Devices = snapshot.Devices[:2]
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 14})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	for _, want := range []string{"GPU 0", "GPU 1", "Tensor Pipe 92%", "DRAM 71%", "Tensor Pipe 63%", "DRAM 51%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("compact tensor wall missing primary metric %q in:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"  SM 84%", "  FP32 33%", "  source NVML+DCGM", "... 1 more GPU(s)"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("compact tensor wall should drop secondary context before primary blocks, found %q in:\n%s", unwanted, view)
		}
	}
}

func TestRenderTensorWallExactFitShowsPrimaryBlockBeforeOverflowMarker(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	snapshot := snapshotWithTensorActivity()
	model, _ = updateModel(model, SnapshotMsg(snapshot))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 13})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) != model.height {
		t.Fatalf("tensor wall line count = %d, want exact height %d:\n%s", len(lines), model.height, view)
	}
	for _, want := range []string{"GPU 0", "GPU 1", "Tensor Pipe 63%", "DRAM 51%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("exact-fit compact tensor wall missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "... 2 more GPU(s)") {
		t.Fatalf("exact-fit compact tensor wall replaced a fitting GPU block with overflow:\n%s", view)
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
	for _, want := range []string{"Tensor/DRAM Activity Wall", "GPU 0", "Tensor Pipe 92%", "DRAM 71%"} {
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

func assertPlainLineColored(t *testing.T, view, plainContains string) {
	t.Helper()
	line := lineByPlainContains(t, view, plainContains)
	if !strings.Contains(line, "\x1b[") {
		t.Fatalf("line containing %q is not colored:\nline: %q\nview:\n%s", plainContains, line, view)
	}
	if plain := stripANSI(line); !strings.Contains(plain, plainContains) {
		t.Fatalf("stripped colored line = %q, want text containing %q", plain, plainContains)
	}
}

func lineByPlainContains(t *testing.T, view, plainContains string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(stripANSI(line), plainContains) {
			return line
		}
	}
	t.Fatalf("view missing line containing %q after stripping ANSI:\n%s", plainContains, view)
	return ""
}

func lineByPlainEqual(t *testing.T, view, plain string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if stripANSI(line) == plain {
			return line
		}
	}
	t.Fatalf("view missing line equal to %q after stripping ANSI:\n%s", plain, view)
	return ""
}

func assertColoredSegmentOccurrence(t *testing.T, line, segment string, occurrence int) {
	t.Helper()
	plainRunes, styled := styledPlainRunes(line)
	segmentRunes := []rune(segment)
	start := nthRuneOccurrence(plainRunes, segmentRunes, occurrence)
	if start < 0 {
		t.Fatalf("line %q missing occurrence %d of segment %q after stripping ANSI", stripANSI(line), occurrence, segment)
	}
	for i, r := range segmentRunes {
		if r == ' ' || r == '\t' {
			continue
		}
		if !styled[start+i] {
			t.Fatalf("segment occurrence %d of %q is not fully colored:\nrendered: %q\nplain:    %q", occurrence, segment, line, stripANSI(line))
		}
	}
}

func styledPlainRunes(value string) ([]rune, []bool) {
	plain := make([]rune, 0, utf8.RuneCountInString(value))
	styled := make([]bool, 0, utf8.RuneCountInString(value))
	active := false
	for i := 0; i < len(value); {
		if value[i] == '\x1b' && i+1 < len(value) && value[i+1] == '[' {
			i += 2
			start := i
			for i < len(value) && (value[i] < '@' || value[i] > '~') {
				i++
			}
			if i < len(value) {
				if value[i] == 'm' {
					active = sgrActiveAfter(active, value[start:i])
				}
				i++
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		plain = append(plain, r)
		styled = append(styled, active)
		i += size
	}
	return plain, styled
}

func sgrActiveAfter(active bool, params string) bool {
	if params == "" {
		return false
	}
	for _, param := range strings.Split(params, ";") {
		switch param {
		case "", "0":
			active = false
		default:
			active = true
		}
	}
	return active
}

func nthRuneOccurrence(value, segment []rune, occurrence int) int {
	if occurrence <= 0 || len(segment) == 0 || len(segment) > len(value) {
		return -1
	}
	seen := 0
	for i := 0; i <= len(value)-len(segment); i++ {
		match := true
		for j := range segment {
			if value[i+j] != segment[j] {
				match = false
				break
			}
		}
		if match {
			seen++
			if seen == occurrence {
				return i
			}
		}
	}
	return -1
}

func stripANSI(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b != '\x1b' {
			builder.WriteByte(b)
			continue
		}
		if i+1 >= len(value) {
			continue
		}
		i++
		if value[i] != '[' {
			continue
		}
		for i+1 < len(value) {
			i++
			if value[i] >= '@' && value[i] <= '~' {
				break
			}
		}
	}
	return builder.String()
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
