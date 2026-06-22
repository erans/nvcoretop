# nvcoretop Core Activity Tiles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render DCGM activity in the expanded GPU detail pane as a two-by-two tile-block visual.

**Architecture:** Keep the change inside `internal/ui/cores.go`, preserving `CoresView(device, preferDCGM)` as the renderer called by the detail pane. The compact overview continues to call `overviewCoresCell` and remains single-line.

**Tech Stack:** Go, existing `gpu.Optional` values, standard library string formatting, existing TUI unit tests.

---

## File Structure

- Modify `internal/ui/cores_test.go`: add red tests for tile-block rendering and missing DCGM values.
- Modify `internal/ui/cores.go`: replace the DCGM branch of `CoresView` with tile-block rendering helpers.
- Verify `internal/ui/render_test.go`: existing overview regression test must continue passing.

---

### Task 1: Add Failing Tile Renderer Tests

**Files:**
- Modify: `internal/ui/cores_test.go`

- [ ] **Step 1: Add tile-block tests**

Replace `TestCoresViewUsesDCGMActivity` in `internal/ui/cores_test.go` with:

```go
func TestCoresViewUsesDCGMTileBlocks(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct:      gpu.Some(83.0),
		TensorActivePct:  gpu.Some(42.0),
		MemPipeActivePct: gpu.Some(58.0),
		FP32ActivePct:    gpu.Some(16.0),
	}, true)

	for _, want := range []string{
		"Core Activity",
		"SM 83%",
		"Tensor 42%",
		"MemPipe 58%",
		"FP32 16%",
		"██████████░░  █████░░░░░░░",
		"███████░░░░░  ██░░░░░░░░░░",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView DCGM tiles = %q, missing %q", got, want)
		}
	}
}

func TestCoresViewShowsMissingDCGMFieldsAsEmptyTiles(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct: gpu.Some(50.0),
	}, true)

	for _, want := range []string{
		"SM 50%",
		"Tensor n/a",
		"MemPipe n/a",
		"FP32 n/a",
		"██████░░░░░░  ░░░░░░░░░░░░",
		"░░░░░░░░░░░░  ░░░░░░░░░░░░",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView missing DCGM tiles = %q, missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify red**

Run:

```bash
go test ./internal/ui -run 'TestCoresViewUsesDCGMTileBlocks|TestCoresViewShowsMissingDCGMFieldsAsEmptyTiles' -v
```

Expected: fail because the current renderer has no `Core Activity` heading and no tile-block output.

---

### Task 2: Implement Tile Renderer

**Files:**
- Modify: `internal/ui/cores.go`

- [ ] **Step 1: Replace DCGM branch with tile rendering**

Update `internal/ui/cores.go` so the imports include `math`:

```go
import (
	"fmt"
	"math"
	"strings"

	"nvcoretop/internal/gpu"
)
```

Then update the file so it contains these helpers and `CoresView` shape:

```go
const (
	coreTileWidth  = 12
	coreTileHeight = 2
)

func CoresView(device gpu.DeviceSample, preferDCGM bool) string {
	if preferDCGM && device.SMActivePct.OK {
		leftTop := activityTile("SM", device.SMActivePct)
		rightTop := activityTile("Tensor", device.TensorActivePct)
		leftBottom := activityTile("MemPipe", device.MemPipeActivePct)
		rightBottom := activityTile("FP32", device.FP32ActivePct)

		lines := []string{"Core Activity"}
		lines = append(lines, combineTiles(leftTop, rightTop)...)
		lines = append(lines, combineTiles(leftBottom, rightBottom)...)
		return strings.Join(lines, "\n")
	}
	return fmt.Sprintf("cores %s %s", percentText(device.GPUUtil), bar(percentFloat(device.GPUUtil), 12))
}

func activityTile(label string, value gpu.Optional[float64]) []string {
	return []string{
		fmt.Sprintf("%-7s %s", label, percentFloatText(value)),
		tileRow(value),
		tileRow(value),
	}
}

func combineTiles(left, right []string) []string {
	lines := make([]string, 0, len(left))
	for i := range left {
		lines = append(lines, fmt.Sprintf("%-20s  %s", left[i], right[i]))
	}
	return lines
}

func tileRow(value gpu.Optional[float64]) string {
	filled := 0
	if value.OK {
		percent := optionalFloatPercent(value)
		filled = int(math.Round((percent / 100) * float64(coreTileWidth)))
		if filled < 0 {
			filled = 0
		}
		if filled > coreTileWidth {
			filled = coreTileWidth
		}
	}
	empty := coreTileWidth - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}
```

- [ ] **Step 2: Run focused tests and verify green**

Run:

```bash
go test ./internal/ui -run 'TestCoresView|TestRenderOverviewDCGMCoresStaySingleLine' -v
```

Expected: pass.

- [ ] **Step 3: Format changed Go files**

Run:

```bash
gofmt -w internal/ui/cores.go internal/ui/cores_test.go
```

Expected: files formatted with no command output.

---

### Task 3: Final Verification, Commit, Push

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run full verification**

Run:

```bash
go mod tidy -diff
git diff --check
go test ./...
go test -tags dcgm ./...
go build ./cmd/nvcoretop
```

Expected: all commands exit 0.

- [ ] **Step 2: Inspect diff**

Run:

```bash
git diff -- internal/ui/cores.go internal/ui/cores_test.go
git status -sb
```

Expected: only the planned renderer/test files are modified after the spec and plan commits.

- [ ] **Step 3: Commit implementation**

Run:

```bash
git add internal/ui/cores.go internal/ui/cores_test.go
git commit -m "feat: render dcgm core activity tiles"
```

Expected: commit succeeds.

- [ ] **Step 4: Push and verify CI**

Run:

```bash
git push origin main
```

Expected: push succeeds. Then use `gh run list` and `gh run watch` to confirm the push-triggered CI run completes successfully.
