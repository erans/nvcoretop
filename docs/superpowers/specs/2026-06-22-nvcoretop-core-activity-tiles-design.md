# nvcoretop Core Activity Tiles Design

**Date:** 2026-06-22
**Status:** Approved for implementation

## Summary

Improve the expanded GPU detail view so real DCGM activity reads as a visual
core-activity tile block instead of four plain horizontal bars.

The visual direction is the approved tile-block layout:

```text
Core Activity
SM 83%        Tensor 42%
██████████░░  █████░░░░░░░
██████████░░  █████░░░░░░░
MemPipe 58%   FP32 16%
███████░░░░░  ██░░░░░░░░░░
███████░░░░░  ██░░░░░░░░░░
```

The visualization shows live aggregate activity percentages from DCGM. It does
not claim to identify exact physical cores or tensor-core units.

## Goals

- Show SM, Tensor, MemPipe, and FP32 activity in the expanded detail pane as
  two-by-two tile blocks.
- Use live DCGM activity fields when available.
- Keep the compact overview single-line and unchanged.
- Keep the current NVML-only fallback for builds or hosts without DCGM.
- Render missing DCGM fields as `n/a` with an empty tile block.
- Keep output stable and deterministic for unit tests.

## Non-goals

- Literal per-core identity mapping.
- New DCGM fields.
- Color-only semantics.
- Changes to JSON or CSV export fields.
- Changes to compact overview layout.

## UI Behavior

When the selected GPU has DCGM activity and the detail pane is open, the detail
view renders:

- A `Core Activity` heading.
- Top row labels for `SM` and `Tensor`.
- Two tile rows for `SM` and `Tensor`.
- Second row labels for `MemPipe` and `FP32`.
- Two tile rows for `MemPipe` and `FP32`.

Each mini tile block is 12 columns wide by 2 rows high. Filled cells scale from
the activity percentage, rounded to the nearest whole tile count. Missing
values render as `n/a` and all empty cells.

When DCGM activity is unavailable, the existing fallback remains:

```text
cores 64% ███████░░░░░
```

## Implementation Notes

- `internal/ui/cores.go` owns the visual renderer.
- Existing `CoresView(device, preferDCGM)` remains the public UI helper used by
  the detail pane.
- `activityBar` is replaced or narrowed so `CoresView` emits the tile-block
  shape when `preferDCGM` is true and `SMActivePct` is present.
- `internal/ui/render.go` continues calling `CoresView` in the detail pane.
- `overviewCoresCell` remains single-line and continues showing only SM
  percentage when DCGM data is present.

## Testing

- Add a unit test for the tile-block detail output with SM/Tensor/MemPipe/FP32
  values.
- Add a unit test for missing optional DCGM fields rendering as `n/a` and empty
  tiles.
- Keep the existing overview test that rejects Tensor/MemPipe/FP32 in the
  compact overview.
- Run the full Go test suite and DCGM-tagged test suite.
