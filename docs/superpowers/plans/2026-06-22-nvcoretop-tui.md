# nvcoretop TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Bubble Tea terminal UI with multi-GPU overview, selected-card detail pane, sparklines, cores visualization, sorting, pause, help, and degraded small-terminal layout.

**Architecture:** `internal/ui` owns state transitions and rendering from `gpu.Snapshot` plus `history.Store`; it does not call NVML or DCGM. The command entrypoint dispatches to `ui.Run` for interactive mode after export mode is already working.

**Tech Stack:** Bubble Tea v1.3.10, Lipgloss v1.1.0, Bubbles v1.0.0, Charm teatest pseudo-version `v0.0.0-20260621010513-945fab64fd3e`, Go unit tests.

---

## File Structure

- Modify: `go.mod` / `go.sum` - add Bubble Tea, Lipgloss, Bubbles, and teatest dependencies.
- Create: `internal/ui/sort.go` - sort modes and stable device ordering.
- Create: `internal/ui/sort_test.go` - sort behavior tests.
- Create: `internal/ui/model.go` - Bubble Tea model, messages, key handling, sizing, and state.
- Create: `internal/ui/model_test.go` - update tests for selection, detail, sort cycling, pause, help, DCGM toggle, and quit.
- Create: `internal/ui/sparkline.go` - sparkline renderer for rolling history.
- Create: `internal/ui/sparkline_test.go` - deterministic sparkline tests.
- Create: `internal/ui/cores.go` - representational core grid and DCGM activity bars.
- Create: `internal/ui/cores_test.go` - tests NVML fallback and DCGM mode.
- Create: `internal/ui/style.go` - color thresholds and no-color style handling.
- Create: `internal/ui/render.go` - root view, overview, detail, footer, and degraded layout renderers.
- Create: `internal/ui/render_test.go` - string snapshot tests of overview/detail/degraded output.
- Create: `internal/ui/run.go` - Bubble Tea program runner and sampler ticker command wiring.
- Create: `internal/ui/run_test.go` - teatest smoke test using `FakeSampler`.
- Modify: `cmd/nvcoretop/main.go` - dispatch default mode to `ui.Run`.

## Prerequisites

Complete these plans first:

- `docs/superpowers/plans/2026-06-22-nvcoretop-core.md`
- `docs/superpowers/plans/2026-06-22-nvcoretop-export-cli.md`

## Decisions Locked In

- Temperature thresholds: normal under 70 C, warning 70-79 C, hot at 80 C and above.
- Utilization thresholds: idle under 25%, active 25-74%, high at 75% and above.
- Power near-limit threshold: warning at 90% of power limit and above.
- Small-terminal degraded layout activates when width is below 72 columns.
- History window: 120 samples, matching two minutes at the default 1s interval.
- No-color mode disables ANSI styling but keeps ASCII labels and Unicode meters.

### Task 1: Add TUI Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add dependencies**

Run:

```bash
go get github.com/charmbracelet/bubbletea@v1.3.10
go get github.com/charmbracelet/lipgloss@v1.1.0
go get github.com/charmbracelet/bubbles@v1.0.0
go get github.com/charmbracelet/x/exp/teatest@v0.0.0-20260621010513-945fab64fd3e
go mod tidy
```

Expected: commands succeed and `go.mod` contains the four dependencies.

- [ ] **Step 2: Verify existing tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add tui dependencies"
```

### Task 2: Add Sort Modes

**Files:**
- Create: `internal/ui/sort_test.go`
- Create: `internal/ui/sort.go`

- [ ] **Step 1: Write sort tests**

Create `internal/ui/sort_test.go` with:

```go
package ui

import (
	"testing"

	"nvcoretop/internal/gpu"
)

func TestSortDevicesByUtilDescending(t *testing.T) {
	devices := []gpu.DeviceSample{
		{Index: 0, GPUUtil: gpu.Some(uint32(10))},
		{Index: 1, GPUUtil: gpu.Some(uint32(90))},
		{Index: 2},
	}

	got := SortDevices(devices, SortUtil)
	if got[0].Index != 1 || got[1].Index != 0 || got[2].Index != 2 {
		t.Fatalf("sort by util order = %#v", indexes(got))
	}
}

func TestSortDevicesByIndexAscending(t *testing.T) {
	devices := []gpu.DeviceSample{{Index: 2}, {Index: 0}, {Index: 1}}
	got := SortDevices(devices, SortIndex)
	if got[0].Index != 0 || got[1].Index != 1 || got[2].Index != 2 {
		t.Fatalf("sort by index order = %#v", indexes(got))
	}
}

func TestNextSortCycles(t *testing.T) {
	if got := SortIndex.Next(); got != SortUtil {
		t.Fatalf("SortIndex.Next() = %v, want SortUtil", got)
	}
	if got := SortPower.Next(); got != SortIndex {
		t.Fatalf("SortPower.Next() = %v, want SortIndex", got)
	}
}

func indexes(devices []gpu.DeviceSample) []int {
	out := make([]int, len(devices))
	for i, device := range devices {
		out[i] = device.Index
	}
	return out
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/ui -run 'TestSortDevices|TestNextSort' -v`

Expected: FAIL with undefined `SortDevices`, `SortIndex`, `SortUtil`, and `SortPower`.

- [ ] **Step 3: Implement sorting**

Create `internal/ui/sort.go` with:

```go
package ui

import (
	"sort"

	"nvcoretop/internal/gpu"
)

type SortMode int

const (
	SortIndex SortMode = iota
	SortUtil
	SortTemp
	SortMem
	SortPower
)

func (m SortMode) Next() SortMode {
	switch m {
	case SortIndex:
		return SortUtil
	case SortUtil:
		return SortTemp
	case SortTemp:
		return SortMem
	case SortMem:
		return SortPower
	default:
		return SortIndex
	}
}

func (m SortMode) String() string {
	switch m {
	case SortUtil:
		return "util"
	case SortTemp:
		return "temp"
	case SortMem:
		return "mem"
	case SortPower:
		return "power"
	default:
		return "index"
	}
}

func SortDevices(devices []gpu.DeviceSample, mode SortMode) []gpu.DeviceSample {
	out := make([]gpu.DeviceSample, len(devices))
	copy(out, devices)
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		switch mode {
		case SortUtil:
			return optionalUint32Desc(left.GPUUtil, right.GPUUtil, left.Index, right.Index)
		case SortTemp:
			return optionalUint32Desc(left.TempC, right.TempC, left.Index, right.Index)
		case SortMem:
			return ratioDesc(left.MemUsed, left.MemTotal, right.MemUsed, right.MemTotal, left.Index, right.Index)
		case SortPower:
			return optionalFloatDesc(left.PowerW, right.PowerW, left.Index, right.Index)
		default:
			return left.Index < right.Index
		}
	})
	return out
}

func optionalUint32Desc(left, right gpu.Optional[uint32], leftIndex, rightIndex int) bool {
	if left.OK != right.OK {
		return left.OK
	}
	if left.Value == right.Value {
		return leftIndex < rightIndex
	}
	return left.Value > right.Value
}

func optionalFloatDesc(left, right gpu.Optional[float64], leftIndex, rightIndex int) bool {
	if left.OK != right.OK {
		return left.OK
	}
	if left.Value == right.Value {
		return leftIndex < rightIndex
	}
	return left.Value > right.Value
}

func ratioDesc(leftUsed, leftTotal, rightUsed, rightTotal uint64, leftIndex, rightIndex int) bool {
	if leftTotal == 0 && rightTotal == 0 {
		return leftIndex < rightIndex
	}
	if leftTotal == 0 {
		return false
	}
	if rightTotal == 0 {
		return true
	}
	left := float64(leftUsed) / float64(leftTotal)
	right := float64(rightUsed) / float64(rightTotal)
	if left == right {
		return leftIndex < rightIndex
	}
	return left > right
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/ui -run 'TestSortDevices|TestNextSort' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sort.go internal/ui/sort_test.go
git commit -m "feat: add gpu sort modes"
```

### Task 3: Add Bubble Tea Model and Key Handling

**Files:**
- Create: `internal/ui/model_test.go`
- Create: `internal/ui/model.go`

- [ ] **Step 1: Write model update tests**

Create `internal/ui/model_test.go` with:

```go
package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"nvcoretop/internal/gpu"
)

func TestModelSelectionKeys(t *testing.T) {
	model := NewModel(Options{})
	model.snapshot = snapshotWithDevices(3)

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyDown})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if model.selected != 2 {
		t.Fatalf("selected = %d, want 2", model.selected)
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyUp})
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}
}

func TestModelToggleKeys(t *testing.T) {
	model := NewModel(Options{})

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyEnter})
	if !model.detail {
		t.Fatalf("detail = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !model.paused {
		t.Fatalf("paused = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !model.help {
		t.Fatalf("help = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if !model.dcgmView {
		t.Fatalf("dcgmView = false, want true")
	}
}

func TestModelSnapshotMessageUpdatesHistory(t *testing.T) {
	model := NewModel(Options{HistoryWindow: 4})
	msg := SnapshotMsg(snapshotWithDevices(1))

	model, _ = updateModel(model, msg)
	if len(model.snapshot.Devices) != 1 {
		t.Fatalf("snapshot devices = %d, want 1", len(model.snapshot.Devices))
	}
	history, ok := model.history.Device(0)
	if !ok || len(history.Util.Values()) != 1 {
		t.Fatalf("history missing util value")
	}
}

func snapshotWithDevices(count int) gpu.Snapshot {
	devices := make([]gpu.DeviceSample, count)
	for i := range devices {
		devices[i] = gpu.DeviceSample{
			Index:    i,
			Name:     "RTX",
			MemUsed:  uint64(i+1) * 1024,
			MemTotal: 8192,
			GPUUtil:  gpu.Some(uint32(10 + i)),
			TempC:    gpu.Some(uint32(40 + i)),
			PowerW:   gpu.Some(float64(100 + i)),
		}
	}
	return gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC(), Source: gpu.SourceNVML, Devices: devices}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/ui -run TestModel -v`

Expected: FAIL with undefined `NewModel`, `Options`, `updateModel`, and `SnapshotMsg`.

- [ ] **Step 3: Implement model**

Create `internal/ui/model.go` with:

```go
package ui

import (
	"nvcoretop/internal/gpu"
	"nvcoretop/internal/history"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultHistoryWindow = 120

type Options struct {
	Interval      string
	NoColor       bool
	HistoryWindow int
	ForceDCGMView bool
}

type SnapshotMsg gpu.Snapshot

type ErrMsg struct {
	Err error
}

type Model struct {
	snapshot gpu.Snapshot
	history  *history.Store
	selected int
	detail   bool
	paused   bool
	help     bool
	dcgmView bool
	sort     SortMode
	width    int
	height   int
	err      error
	options  Options
}

func NewModel(options Options) Model {
	window := options.HistoryWindow
	if window <= 0 {
		window = defaultHistoryWindow
	}
	return Model{
		history:  history.NewStore(window),
		dcgmView: options.ForceDCGMView,
		sort:     SortIndex,
		options:  options,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := updateModel(m, msg)
	return next, cmd
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case SnapshotMsg:
		m.snapshot = gpu.Snapshot(msg)
		m.history.Add(m.snapshot)
		m.clampSelection()
	case ErrMsg:
		m.err = msg.Err
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			m.selected++
			m.clampSelection()
		case "up", "k":
			m.selected--
			m.clampSelection()
		case "enter", "tab":
			m.detail = !m.detail
		case "s":
			m.sort = m.sort.Next()
			m.clampSelection()
		case "d":
			m.dcgmView = !m.dcgmView
		case "p":
			m.paused = !m.paused
		case "?":
			m.help = !m.help
		}
	}
	return m, nil
}

func (m *Model) clampSelection() {
	count := len(m.snapshot.Devices)
	if count == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= count {
		m.selected = count - 1
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/ui -run TestModel -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: add tui model state"
```

### Task 4: Add Sparkline Renderer

**Files:**
- Create: `internal/ui/sparkline_test.go`
- Create: `internal/ui/sparkline.go`

- [ ] **Step 1: Write sparkline tests**

Create `internal/ui/sparkline_test.go` with:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/ui -run TestSparkline -v`

Expected: FAIL with undefined `Sparkline`.

- [ ] **Step 3: Implement sparkline**

Create `internal/ui/sparkline.go` with:

```go
package ui

import "strings"

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return "n/a"
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}

	min := values[0]
	max := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	var builder strings.Builder
	for _, value := range values {
		index := 0
		if max > min {
			index = int(((value - min) / (max - min)) * float64(len(sparkRunes)-1))
		}
		builder.WriteRune(sparkRunes[index])
	}
	return builder.String()
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/ui -run TestSparkline -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sparkline.go internal/ui/sparkline_test.go
git commit -m "feat: render metric sparklines"
```

### Task 5: Add Cores Visualization

**Files:**
- Create: `internal/ui/cores_test.go`
- Create: `internal/ui/cores.go`

- [ ] **Step 1: Write cores tests**

Create `internal/ui/cores_test.go` with:

```go
package ui

import (
	"strings"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestCoresGridUsesGPUUtilFallback(t *testing.T) {
	got := CoresView(gpu.DeviceSample{GPUUtil: gpu.Some(uint32(50))}, false)
	if !strings.Contains(got, "cores") || !strings.Contains(got, "50%") {
		t.Fatalf("CoresView fallback = %q", got)
	}
}

func TestCoresViewUsesDCGMActivity(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct:      gpu.Some(82.5),
		TensorActivePct:  gpu.Some(12.0),
		MemPipeActivePct: gpu.Some(44.0),
		FP32ActivePct:    gpu.Some(65.0),
	}, true)
	for _, want := range []string{"SM", "Tensor", "MemPipe", "FP32"} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView DCGM = %q, missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/ui -run TestCores -v`

Expected: FAIL with undefined `CoresView`.

- [ ] **Step 3: Implement cores rendering**

Create `internal/ui/cores.go` with:

```go
package ui

import (
	"fmt"
	"strings"

	"nvcoretop/internal/gpu"
)

func CoresView(device gpu.DeviceSample, preferDCGM bool) string {
	if preferDCGM && device.SMActivePct.OK {
		lines := []string{
			activityBar("SM", device.SMActivePct),
			activityBar("Tensor", device.TensorActivePct),
			activityBar("MemPipe", device.MemPipeActivePct),
			activityBar("FP32", device.FP32ActivePct),
		}
		return strings.Join(lines, "\n")
	}
	return fmt.Sprintf("cores %s %s", percentText(device.GPUUtil), bar(percentFloat(device.GPUUtil), 12))
}

func activityBar(label string, value gpu.Optional[float64]) string {
	return fmt.Sprintf("%-7s %6s %s", label, percentFloatText(value), bar(optionalFloatPercent(value), 16))
}

func bar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func percentText(value gpu.Optional[uint32]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d%%", value.Value)
}

func percentFloat(value gpu.Optional[uint32]) float64 {
	if !value.OK {
		return 0
	}
	return float64(value.Value)
}

func percentFloatText(value gpu.Optional[float64]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", value.Value)
}

func optionalFloatPercent(value gpu.Optional[float64]) float64 {
	if !value.OK {
		return 0
	}
	return value.Value
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/ui -run TestCores -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/cores.go internal/ui/cores_test.go
git commit -m "feat: render gpu core activity"
```

### Task 6: Add Styling and Root Renderers

**Files:**
- Create: `internal/ui/style.go`
- Create: `internal/ui/render_test.go`
- Create: `internal/ui/render.go`

- [ ] **Step 1: Create style helpers**

Create `internal/ui/style.go` with:

```go
package ui

import "github.com/charmbracelet/lipgloss"

type palette struct {
	muted lipgloss.Style
	warn  lipgloss.Style
	hot   lipgloss.Style
	ok    lipgloss.Style
}

func styles(noColor bool) palette {
	if noColor {
		return palette{}
	}
	return palette{
		muted: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		warn:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		hot:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		ok:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
	}
}

func tempState(temp uint32) string {
	switch {
	case temp >= 80:
		return "hot"
	case temp >= 70:
		return "warn"
	default:
		return "ok"
	}
}

func utilState(util uint32) string {
	switch {
	case util >= 75:
		return "high"
	case util >= 25:
		return "active"
	default:
		return "idle"
	}
}

func powerNearLimit(power, limit float64) bool {
	return limit > 0 && power/limit >= 0.90
}
```

- [ ] **Step 2: Write render tests**

Create `internal/ui/render_test.go` with:

```go
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
```

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/ui -run TestRender -v`

Expected: FAIL with missing `View` render behavior.

- [ ] **Step 4: Implement renderers**

Create `internal/ui/render.go` with:

```go
package ui

import (
	"fmt"
	"strings"

	"nvcoretop/internal/gpu"
)

const degradedWidth = 72

func (m Model) View() string {
	var parts []string
	if m.err != nil {
		parts = append(parts, "error: "+m.err.Error())
	}
	if m.width > 0 && m.width < degradedWidth {
		parts = append(parts, m.renderDegraded())
	} else {
		parts = append(parts, m.renderOverview())
	}
	if m.detail && len(m.snapshot.Devices) > 0 {
		parts = append(parts, m.renderDetail(m.selectedDevice()))
	}
	parts = append(parts, m.renderFooter())
	if m.help {
		parts = append(parts, "keys: up/down/j/k select | enter/tab detail | s sort | d dcgm | p pause | ? help | q quit")
	}
	return strings.Join(parts, "\n")
}

func (m Model) selectedDevice() gpu.DeviceSample {
	devices := SortDevices(m.snapshot.Devices, m.sort)
	if len(devices) == 0 {
		return gpu.DeviceSample{}
	}
	if m.selected >= len(devices) {
		return devices[len(devices)-1]
	}
	return devices[m.selected]
}

func (m Model) renderOverview() string {
	lines := []string{" #  NAME        UTIL        MEM             TEMP   PWR        CORES"}
	for row, device := range SortDevices(m.snapshot.Devices, m.sort) {
		cursor := " "
		if row == m.selected {
			cursor = ">"
		}
		lines = append(lines, fmt.Sprintf("%s%2d  %-10.10s %-10s %-14s %-6s %-10s %s",
			cursor,
			device.Index,
			device.Name,
			utilCell(device),
			memCell(device),
			tempCell(device),
			powerCell(device),
			CoresView(device, m.dcgmView || m.snapshot.Source == gpu.SourceNVMLDCGM),
		))
	}
	if len(m.snapshot.Devices) == 0 {
		lines = append(lines, "waiting for GPU samples...")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDegraded() string {
	lines := make([]string, 0, len(m.snapshot.Devices))
	for _, device := range SortDevices(m.snapshot.Devices, m.sort) {
		lines = append(lines, fmt.Sprintf("GPU %d %s  util %s  mem %s  temp %s  pwr %s",
			device.Index,
			device.Name,
			percentText(device.GPUUtil),
			memCell(device),
			tempCell(device),
			powerCell(device),
		))
	}
	if len(lines) == 0 {
		return "waiting for GPU samples..."
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail(device gpu.DeviceSample) string {
	history, _ := m.history.Device(device.Index)
	lines := []string{
		fmt.Sprintf("Detail GPU %d %s", device.Index, device.Name),
		fmt.Sprintf("Util   %s", Sparkline(history.Util.Values(), 32)),
		fmt.Sprintf("Temp   %s", Sparkline(history.Temp.Values(), 32)),
		fmt.Sprintf("Power  %s", Sparkline(history.Power.Values(), 32)),
		fmt.Sprintf("Clocks SM %s MHz  MEM %s MHz", optionalUint(device.SMClockMHz), optionalUint(device.MemClockMHz)),
		fmt.Sprintf("Throttle %s", throttleText(device.ThrottleReasons)),
		fmt.Sprintf("Fan %s", optionalUint(device.FanPct)),
		"Processes",
		processTable(device),
		fmt.Sprintf("PCIe tx %s KB/s rx %s KB/s", optionalUint64Text(device.PCIeTxKBps), optionalUint64Text(device.PCIeRxKBps)),
		fmt.Sprintf("NVLink tx %s KB/s rx %s KB/s", optionalUint64Text(device.NVLinkTxKBps), optionalUint64Text(device.NVLinkRxKBps)),
		fmt.Sprintf("ECC single %s double %s", optionalUint64Text(device.ECCSingleBit), optionalUint64Text(device.ECCDoubleBit)),
		CoresView(device, m.dcgmView || m.snapshot.Source == gpu.SourceNVMLDCGM),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFooter() string {
	status := "running"
	if m.paused {
		status = "paused"
	}
	return fmt.Sprintf("%s | sort %s | source %s", status, m.sort.String(), m.snapshot.Source.String())
}

func utilCell(device gpu.DeviceSample) string {
	if !device.GPUUtil.OK {
		return "n/a"
	}
	return fmt.Sprintf("%s %3d%%", bar(float64(device.GPUUtil.Value), 6), device.GPUUtil.Value)
}

func memCell(device gpu.DeviceSample) string {
	if device.MemTotal == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f/%.1f GB", bytesToGB(device.MemUsed), bytesToGB(device.MemTotal))
}

func tempCell(device gpu.DeviceSample) string {
	if !device.TempC.OK {
		return "n/a"
	}
	marker := ""
	if device.ThrottleReasons.Active() {
		marker = "^"
	}
	return fmt.Sprintf("%dC%s", device.TempC.Value, marker)
}

func powerCell(device gpu.DeviceSample) string {
	if !device.PowerW.OK {
		return "n/a"
	}
	if device.PowerLimitW.OK {
		return fmt.Sprintf("%.0f/%.0fW", device.PowerW.Value, device.PowerLimitW.Value)
	}
	return fmt.Sprintf("%.0fW", device.PowerW.Value)
}

func optionalUint(value gpu.Optional[uint32]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d", value.Value)
}

func optionalUint64Text(value gpu.Optional[uint64]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d", value.Value)
}

func throttleText(reasons gpu.ThrottleReasons) string {
	if !reasons.Active() {
		return "none"
	}
	return strings.Join(reasons.Names(), ",")
}

func processTable(device gpu.DeviceSample) string {
	if device.ProcessLimited {
		return "process query permission-limited"
	}
	if len(device.Processes) == 0 {
		return "no processes"
	}
	lines := []string{"PID      NAME                  VRAM"}
	for _, proc := range device.Processes {
		lines = append(lines, fmt.Sprintf("%-8d %-20.20s %.1f GB", proc.PID, proc.Name, bytesToGB(proc.MemUsed)))
	}
	return strings.Join(lines, "\n")
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}
```

- [ ] **Step 5: Run render tests**

Run: `go test ./internal/ui -run TestRender -v`

Expected: PASS.

- [ ] **Step 6: Run all UI tests**

Run: `go test ./internal/ui -v`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/style.go internal/ui/render.go internal/ui/render_test.go
git commit -m "feat: render nvcoretop tui"
```

### Task 7: Add TUI Runner and CLI Dispatch

**Files:**
- Create: `internal/ui/run.go`
- Create: `internal/ui/run_test.go`
- Modify: `cmd/nvcoretop/main.go`

- [ ] **Step 1: Write teatest smoke test**

Create `internal/ui/run_test.go` with:

```go
package ui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"nvcoretop/internal/gpu"
)

func TestProgramRendersSnapshot(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	tm.Send(SnapshotMsg(gpu.Snapshot{
		Timestamp: time.Unix(1, 0).UTC(),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{
			Index:    0,
			Name:     "RTX 3090",
			MemUsed:  8 * 1024 * 1024 * 1024,
			MemTotal: 24 * 1024 * 1024 * 1024,
			GPUUtil:  gpu.Some(uint32(64)),
			TempC:    gpu.Some(uint32(71)),
			PowerW:   gpu.Some(285.0),
		}},
	}))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSampleCmdSkipsWhenPaused(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: snapshotWithDevices(1)}})
	cmd := sampleCmd(context.Background(), sampler, true)
	if msg := cmd(); msg != nil {
		t.Fatalf("paused sample cmd = %#v, want nil", msg)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/ui -run 'TestProgramRendersSnapshot|TestSampleCmdSkipsWhenPaused' -v`

Expected: FAIL with undefined `sampleCmd`.

- [ ] **Step 3: Implement runner**

Create `internal/ui/run.go` with:

```go
package ui

import (
	"context"
	"time"

	"nvcoretop/internal/gpu"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, sampler gpu.Sampler, interval time.Duration, options Options) error {
	model := NewModel(options)
	program := tea.NewProgram(model, tea.WithAltScreen())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				program.Quit()
				return
			case <-ticker.C:
				program.Send(sampleCmd(ctx, sampler, false)())
			}
		}
	}()
	_, err := program.Run()
	return err
}

func sampleCmd(ctx context.Context, sampler gpu.Sampler, paused bool) tea.Cmd {
	return func() tea.Msg {
		if paused {
			return nil
		}
		snapshot, err := sampler.Sample(ctx)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SnapshotMsg(snapshot)
	}
}
```

- [ ] **Step 4: Modify CLI dispatch**

In `cmd/nvcoretop/main.go`, add imports:

```go
	"time"

	"nvcoretop/internal/ui"
```

Replace the default branch in `run` with:

```go
	default:
		sampler := gpu.NewFakeSampler([]gpu.FakeStep{{
			Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
		}})
		defer sampler.Close()
		return ui.Run(context.Background(), sampler, cfg.Interval, ui.Options{
			Interval: cfg.Interval.String(),
			NoColor:  cfg.NoColor,
		})
```

Remove the `fmt.Fprintln(stderr, "interactive TUI mode will be enabled by the UI plan")` line after this replacement. If `time` is not used after editing, do not keep the `time` import.

- [ ] **Step 5: Run UI tests**

Run: `go test ./internal/ui -v`

Expected: PASS.

- [ ] **Step 6: Build binary**

Run: `go build ./cmd/nvcoretop`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/run.go internal/ui/run_test.go cmd/nvcoretop/main.go
git commit -m "feat: wire interactive tui mode"
```

## Self-Review Checklist

- Spec coverage: covers Bubble Tea model/update/view, multi-card overview, selected detail pane, sparklines, cores grid, process table, PCIe/NVLink/ECC rendering, keybindings, footer, pause/help/DCGM toggles, no-color rendering, and degraded small-terminal layout.
- Deferred to later plan: replacing fake sampler with NVML/DCGM factory and real DCGM source detection.
- Unresolved-content scan: every code step provides concrete file content and every test step has an exact command with expected output.
- Type consistency: uses `gpu.Snapshot`, `history.Store`, `SortMode`, `SnapshotMsg`, `Options`, and `ui.Run` consistently.
