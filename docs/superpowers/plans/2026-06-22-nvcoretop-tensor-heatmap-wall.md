# nvcoretop Tensor And DRAM Heatmap Wall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `t`-toggleable all-GPU wall that shows Tensor Pipe and DRAM activity together with readable color and no-color output.

**Architecture:** Keep the feature inside `internal/ui`. Add a small view-mode field to `Model`, render the Tensor/DRAM wall from a new focused `tensor_wall.go`, and reuse the existing `gpu.DeviceSample` DCGM fields. `TensorActivePct` is DCGM tensor pipe active; `MemPipeActivePct` is DCGM DRAM active and must be labeled `DRAM` in the UI. Use Lipgloss styles only at render time so `--no-color` remains plain text.

**Tech Stack:** Go 1.24, Bubble Tea model/update flow, Lipgloss styling, existing `internal/gpu` sample types, `go test`.

---

## Scope Check

The approved spec covers one UI feature: a dedicated Tensor Pipe + DRAM activity wall plus visible color styling. It does not require CI, release automation, DCGM collection changes, mouse interaction, or a web UI.

## File Structure

- Modify `internal/ui/model.go`: add explicit UI view mode and key handling for `t` and `o`.
- Modify `internal/ui/model_test.go`: cover tensor wall key behavior.
- Modify `internal/ui/render.go`: branch `View()` into tensor wall rendering, and update help text.
- Create `internal/ui/tensor_wall.go`: render all-GPU Tensor Pipe and DRAM wall blocks and heatmap rows.
- Modify `internal/ui/render_test.go`: cover Tensor/DRAM wall rendering, unavailable Tensor Pipe or DRAM data, no-color behavior, color behavior, and width bounds.
- Modify `internal/ui/style.go`: add style helpers for activity levels and make existing styles visible in rendered UI.

## Task 1: Add Tensor Wall View Mode And Keys

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_test.go`

- [ ] **Step 1: Add failing model key tests**

Append this test to `internal/ui/model_test.go` after `TestModelToggleKeys`:

```go
func TestModelTensorWallKeys(t *testing.T) {
	model := NewModel(Options{})
	model.detail = true

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if model.view != viewTensorWall {
		t.Fatalf("view = %v, want tensor wall", model.view)
	}
	if !model.detail {
		t.Fatalf("detail = false, want preserved while entering tensor wall")
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if model.view != viewOverview {
		t.Fatalf("view = %v, want overview after toggling tensor wall off", model.view)
	}
	if !model.detail {
		t.Fatalf("detail = false, want t toggle to preserve previous detail state")
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if model.view != viewOverview {
		t.Fatalf("view = %v, want overview after o", model.view)
	}
	if model.detail {
		t.Fatalf("detail = true, want o to return to overview without detail")
	}
}
```

- [ ] **Step 2: Run the new test and verify it fails**

Run:

```bash
go test ./internal/ui -run TestModelTensorWallKeys -v
```

Expected: failure to compile because `model.view`, `viewTensorWall`, and `viewOverview` do not exist.

- [ ] **Step 3: Add the view mode and key handling**

In `internal/ui/model.go`, add this type and constants after `type ErrMsg struct`:

```go
type viewMode int

const (
	viewOverview viewMode = iota
	viewTensorWall
)
```

Add this field to `Model` after `sort SortMode`:

```go
	view     viewMode
```

Add these cases inside the `tea.KeyMsg` switch, after the `d` case:

```go
		case "t":
			if m.view == viewTensorWall {
				m.view = viewOverview
			} else {
				m.view = viewTensorWall
			}
		case "o":
			m.view = viewOverview
			m.detail = false
```

The key switch should contain this ordered block:

```go
		case "d":
			m.dcgmView = !m.dcgmView
		case "t":
			if m.view == viewTensorWall {
				m.view = viewOverview
			} else {
				m.view = viewTensorWall
			}
		case "o":
			m.view = viewOverview
			m.detail = false
		case "p":
			m.paused = !m.paused
```

- [ ] **Step 4: Run the focused model tests**

Run:

```bash
go test ./internal/ui -run 'TestModel(TensorWallKeys|ToggleKeys)' -v
```

Expected: both tests pass.

- [ ] **Step 5: Commit the model state change**

Run:

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: add tensor wall view mode"
```

## Task 2: Render Tensor And DRAM Wall In No-Color Mode

**Files:**
- Modify: `internal/ui/render.go`
- Create: `internal/ui/tensor_wall.go`
- Modify: `internal/ui/render_test.go`

- [ ] **Step 1: Add failing render tests and fixtures**

Append these tests and helpers to `internal/ui/render_test.go`:

```go
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
```

- [ ] **Step 2: Run the new render tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestRenderTensorWall' -v
```

Expected: tests fail because `View()` still renders the normal overview when `model.view == viewTensorWall`.

- [ ] **Step 3: Branch `View()` into tensor wall mode**

In `internal/ui/render.go`, update the start of `View()` to this shape:

```go
func (m Model) View() string {
	var parts []string
	degraded := m.width > 0 && m.width < degradedWidth
	if m.err != nil {
		parts = append(parts, "error: "+m.err.Error())
	}
	if m.view == viewTensorWall {
		parts = append(parts, m.renderTensorWall())
		parts = append(parts, m.renderFooter())
		if m.help {
			parts = append(parts, "keys: t tensor wall | o overview | s sort | d dcgm | p pause | ? help | q quit")
		}
		return strings.Join(parts, "\n")
	}
	if degraded {
		parts = append(parts, m.renderDegraded())
	} else {
		parts = append(parts, m.renderOverview())
	}
	if m.detail && len(m.snapshot.Devices) > 0 {
		detail := m.renderDetail(m.selectedDevice())
		if degraded {
			detail = truncateLines(detail, m.width)
		}
		parts = append(parts, detail)
	}
	parts = append(parts, m.renderFooter())
	if m.help {
		parts = append(parts, "keys: up/down/j/k select | enter/tab detail | t tensor wall | o overview | s sort | d dcgm | p pause | ? help | q quit")
	}
	return strings.Join(parts, "\n")
}
```

- [ ] **Step 4: Create the no-color tensor wall renderer**

Create `internal/ui/tensor_wall.go` with this content:

```go
package ui

import (
	"fmt"
	"math"
	"strings"

	"nvcoretop/internal/gpu"
)

const (
	tensorWallDefaultWidth     = 100
	tensorWallDefaultHeight    = 30
	tensorWallMinHeatmapWidth  = 12
	tensorWallMaxHeatmapWidth  = 48
	tensorWallMinHeatmapHeight = 1
	tensorWallMaxHeatmapHeight = 6
)

func (m Model) renderTensorWall() string {
	width := m.width
	if width <= 0 {
		width = tensorWallDefaultWidth
	}
	height := m.height
	if height <= 0 {
		height = tensorWallDefaultHeight
	}

	lines := []string{"Tensor/DRAM Activity Wall"}
	devices := SortDevices(m.snapshot.Devices, m.sort)
	if len(devices) == 0 {
		lines = append(lines, "waiting for GPU samples...")
		return strings.Join(lines, "\n")
	}

	heatWidth := tensorHeatmapWidth(width)
	heatHeight := tensorHeatmapHeight(height, len(devices))
	for row, device := range devices {
		if row > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderTensorGPUBlock(device, m.snapshot.Source, width, heatWidth, heatHeight)...)
	}
	return strings.Join(lines, "\n")
}

func renderTensorGPUBlock(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int) []string {
	name := truncateRunes(device.Name, tensorNameWidth(width))
	lines := []string{
		truncateRunes(fmt.Sprintf(
			"GPU %-2d %-*s Tensor Pipe %s  DRAM %s",
			device.Index,
			tensorNameWidth(width),
			name,
			percentFloatText(device.TensorActivePct),
			percentFloatText(device.MemPipeActivePct),
		), width),
		truncateRunes(fmt.Sprintf(
			"  SM %s  FP32 %s  util %s  mem %s  temp %s  source %s",
			percentFloatText(device.SMActivePct),
			percentFloatText(device.FP32ActivePct),
			percentText(device.GPUUtil),
			memCell(device),
			tempCell(device),
			source.String(),
		), width),
	}

	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight)
	return lines
}

func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int) []string {
	if !value.OK {
		return append(lines, truncateRunes(fmt.Sprintf("  %-11s unavailable (DCGM field missing)", label), width))
	}
	for _, row := range tensorHeatmapRows(value, heatWidth, heatHeight) {
		lines = append(lines, truncateRunes(fmt.Sprintf("  %-11s %s", label, row), width))
	}
	return lines
}

func tensorHeatmapRows(value gpu.Optional[float64], width, height int) []string {
	width = clampInt(width, 1, tensorWallMaxHeatmapWidth)
	height = clampInt(height, 1, tensorWallMaxHeatmapHeight)
	total := width * height
	filled := 0
	if value.OK {
		percent := clampFloat(value.Value, 0, 100)
		filled = int(math.Round((percent / 100) * float64(total)))
	}

	rows := make([]string, 0, height)
	for row := 0; row < height; row++ {
		var builder strings.Builder
		for col := 0; col < width; col++ {
			cell := row*width + col
			if cell < filled {
				builder.WriteRune('█')
			} else {
				builder.WriteRune('░')
			}
		}
		rows = append(rows, builder.String())
	}
	return rows
}

func tensorHeatmapWidth(width int) int {
	if width <= 0 {
		width = tensorWallDefaultWidth
	}
	return clampInt(width-16, tensorWallMinHeatmapWidth, tensorWallMaxHeatmapWidth)
}

func tensorHeatmapHeight(height, gpuCount int) int {
	if height <= 0 {
		height = tensorWallDefaultHeight
	}
	if gpuCount <= 0 {
		return tensorWallMinHeatmapHeight
	}
	available := height - 4
	perGPU := available / gpuCount
	return clampInt((perGPU-3)/2, tensorWallMinHeatmapHeight, tensorWallMaxHeatmapHeight)
}

func tensorNameWidth(width int) int {
	return clampInt(width-76, 8, 28)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
```

- [ ] **Step 5: Run the focused render tests**

Run:

```bash
go test ./internal/ui -run 'TestRenderTensorWall|TestRenderOverviewNoColor|TestRenderDegraded' -v
```

Expected: tensor wall tests pass, and existing overview/degraded tests still pass.

- [ ] **Step 6: Commit the no-color tensor wall renderer**

Run:

```bash
git add internal/ui/render.go internal/ui/tensor_wall.go internal/ui/render_test.go
git commit -m "feat: render tensor heatmap wall"
```

## Task 3: Add Color To Tensor Wall And Existing UI Chrome

**Files:**
- Modify: `internal/ui/style.go`
- Modify: `internal/ui/tensor_wall.go`
- Modify: `internal/ui/render.go`
- Modify: `internal/ui/render_test.go`

- [ ] **Step 1: Add failing color tests**

Append these tests to `internal/ui/render_test.go`:

```go
func TestRenderTensorWallUsesColorWhenEnabled(t *testing.T) {
	model := NewModel(Options{})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithTensorActivity()))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})

	view := model.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("colored tensor wall missing ANSI escapes:\n%s", view)
	}
}

func TestRenderOverviewUsesColorWhenEnabled(t *testing.T) {
	model := NewModel(Options{})
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(2)))
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("colored overview missing ANSI escapes:\n%s", view)
	}
}
```

- [ ] **Step 2: Run the color tests and verify they fail**

Run:

```bash
go test ./internal/ui -run 'TestRender(TensorWallUsesColorWhenEnabled|OverviewUsesColorWhenEnabled)' -v
```

Expected: both tests fail because styles are defined but not applied to visible output.

- [ ] **Step 3: Add palette helpers**

In `internal/ui/style.go`, add this method block after `styles`:

```go
func (p palette) activity(percent float64) lipgloss.Style {
	switch {
	case percent >= 75:
		return p.hot
	case percent >= 50:
		return p.warn
	case percent > 0:
		return p.ok
	default:
		return p.muted
	}
}

func (p palette) optionalActivity(value float64, ok bool) lipgloss.Style {
	if !ok {
		return p.muted
	}
	return p.activity(value)
}
```

- [ ] **Step 4: Apply color in the tensor wall renderer**

Change the `renderTensorWall` title in `internal/ui/tensor_wall.go` from:

```go
	lines := []string{"Tensor/DRAM Activity Wall"}
```

to:

```go
	st := styles(m.options.NoColor)
	lines := []string{st.hot.Render("Tensor/DRAM Activity Wall")}
```

Change the block call from:

```go
		lines = append(lines, renderTensorGPUBlock(device, m.snapshot.Source, width, heatWidth, heatHeight)...)
```

to:

```go
		lines = append(lines, renderTensorGPUBlock(device, m.snapshot.Source, width, heatWidth, heatHeight, st, m.options.NoColor)...)
```

Change the function signature from:

```go
func renderTensorGPUBlock(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int) []string {
```

to:

```go
func renderTensorGPUBlock(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int, st palette, noColor bool) []string {
```

Replace the `lines := []string{...}` block inside `renderTensorGPUBlock` with:

```go
	header := fmt.Sprintf(
		"GPU %-2d %-*s Tensor Pipe %s  DRAM %s",
		device.Index,
		tensorNameWidth(width),
		name,
		st.optionalActivity(optionalFloatPercent(device.TensorActivePct), device.TensorActivePct.OK).Render(percentFloatText(device.TensorActivePct)),
		st.optionalActivity(optionalFloatPercent(device.MemPipeActivePct), device.MemPipeActivePct.OK).Render(percentFloatText(device.MemPipeActivePct)),
	)
	context := fmt.Sprintf(
		"  SM %s  FP32 %s  util %s  mem %s  temp %s  source %s",
		st.optionalActivity(optionalFloatPercent(device.SMActivePct), device.SMActivePct.OK).Render(percentFloatText(device.SMActivePct)),
		st.optionalActivity(optionalFloatPercent(device.FP32ActivePct), device.FP32ActivePct.OK).Render(percentFloatText(device.FP32ActivePct)),
		st.optionalActivity(percentFloat(device.GPUUtil), device.GPUUtil.OK).Render(percentText(device.GPUUtil)),
		memCell(device),
		tempCell(device),
		st.muted.Render(source.String()),
	)
	if noColor {
		header = truncateRunes(header, width)
		context = truncateRunes(context, width)
	}
	lines := []string{header, context}
```

Change `appendActivityHeatmap` from:

```go
func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int) []string {
	if !value.OK {
		return append(lines, truncateRunes(fmt.Sprintf("  %-11s unavailable (DCGM field missing)", label), width))
	}
	for _, row := range tensorHeatmapRows(value, heatWidth, heatHeight) {
		lines = append(lines, truncateRunes(fmt.Sprintf("  %-11s %s", label, row), width))
	}
	return lines
}
```

to:

```go
func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int, st palette, noColor bool) []string {
	labelText := label
	if !noColor {
		labelText = st.muted.Render(label)
	}
	if !value.OK {
		unavailable := fmt.Sprintf("  %-11s %s", labelText, st.muted.Render("unavailable (DCGM field missing)"))
		if noColor {
			unavailable = truncateRunes(unavailable, width)
		}
		return append(lines, unavailable)
	}
	for _, row := range tensorHeatmapRows(value, heatWidth, heatHeight, st) {
		line := fmt.Sprintf("  %-11s %s", labelText, row)
		if noColor {
			line = truncateRunes(line, width)
		}
		lines = append(lines, line)
	}
	return lines
}
```

Change the heatmap calls from:

```go
	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight)
```

to:

```go
	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight, st, noColor)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight, st, noColor)
```

Change the heatmap function signature from:

```go
func tensorHeatmapRows(value gpu.Optional[float64], width, height int) []string {
```

to:

```go
func tensorHeatmapRows(value gpu.Optional[float64], width, height int, st palette) []string {
```

Inside `tensorHeatmapRows`, add this line after `filled := 0`:

```go
	cellStyle := st.muted
```

Inside the `if value.OK` block, add this line after `filled = int(math.Round((percent / 100) * float64(total)))`:

```go
		cellStyle = st.activity(percent)
```

Replace the cell-writing block:

```go
			if cell < filled {
				builder.WriteRune('█')
			} else {
				builder.WriteRune('░')
			}
```

with:

```go
			if cell < filled {
				builder.WriteString(cellStyle.Render("█"))
			} else {
				builder.WriteString(st.muted.Render("░"))
			}
```

- [ ] **Step 5: Apply visible color to overview chrome**

In `internal/ui/render.go`, change `renderOverview` from:

```go
func (m Model) renderOverview() string {
	lines := []string{" #  NAME        UTIL        MEM             TEMP   PWR        CORES"}
```

to:

```go
func (m Model) renderOverview() string {
	st := styles(m.options.NoColor)
	lines := []string{st.muted.Render(" #  NAME        UTIL        MEM             TEMP   PWR        CORES")}
```

Change the cursor assignment from:

```go
		cursor := " "
		if row == m.selected {
			cursor = ">"
		}
```

to:

```go
		cursor := " "
		if row == m.selected {
			cursor = st.ok.Render(">")
		}
```

Change `renderFooter` from:

```go
func (m Model) renderFooter() string {
	status := "running"
	if m.paused {
		status = "paused"
	}
	return fmt.Sprintf("%s | interval %s | sort %s | source %s", status, m.options.Interval, m.sort.String(), m.snapshot.Source.String())
}
```

to:

```go
func (m Model) renderFooter() string {
	st := styles(m.options.NoColor)
	status := st.ok.Render("running")
	if m.paused {
		status = st.warn.Render("paused")
	}
	return fmt.Sprintf("%s | interval %s | sort %s | source %s", status, m.options.Interval, m.sort.String(), m.snapshot.Source.String())
}
```

- [ ] **Step 6: Run focused color and no-color tests**

Run:

```bash
go test ./internal/ui -run 'TestRender(TensorWallUsesColorWhenEnabled|OverviewUsesColorWhenEnabled|TensorWallNoColorMultipleGPUs|OverviewNoColor)' -v
```

Expected: color-enabled tests contain ANSI escapes, and no-color tests contain no ANSI escapes.

- [ ] **Step 7: Run all UI tests**

Run:

```bash
go test ./internal/ui -v
```

Expected: all `internal/ui` tests pass.

- [ ] **Step 8: Commit color styling**

Run:

```bash
git add internal/ui/style.go internal/ui/tensor_wall.go internal/ui/render.go internal/ui/render_test.go
git commit -m "style: colorize tensor wall ui"
```

## Task 4: Full Verification

**Files:**
- Verify only; no planned source edits.

- [ ] **Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: every package reports `ok` or has no tests. Any failure must be investigated before continuing.

- [ ] **Step 2: Run whitespace diff check**

Run:

```bash
git diff --check HEAD
```

Expected: no output and exit code 0.

- [ ] **Step 3: Inspect final commits**

Run:

```bash
git log --oneline -4
```

Expected: the three feature commits appear above the design commit:

```text
style: colorize tensor wall ui
feat: render tensor heatmap wall
feat: add tensor wall view mode
docs: design tensor heatmap wall
```

- [ ] **Step 4: Report status**

Report:

```text
Implemented tensor wall mode behind the `t` key, with `o` returning to overview. Verified with `go test ./...` and `git diff --check HEAD`.
```
