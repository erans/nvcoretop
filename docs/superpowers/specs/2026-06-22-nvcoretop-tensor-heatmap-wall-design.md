# nvcoretop Tensor Heatmap Wall Design

Date: 2026-06-22

## Status

Design approved for implementation planning.

## Goal

Add a dedicated TUI view that makes tensor activity visible across all GPUs at the same time. The first version should prioritize a large, readable tensor heatmap per GPU over fitting the visualization into the existing overview table.

## Context

The existing extended view can show DCGM core activity tiles for the selected GPU, including tensor activity. This is useful for inspecting one GPU, but it does not answer the operational question "which GPUs are doing tensor work right now?" across a multi-GPU host.

The current renderer also defines color styles but does not apply them to the visible UI, so the output looks mostly plain even when the terminal supports color.

## Data Semantics

DCGM exposes aggregate tensor activity per GPU. It does not identify exact physical tensor core instances. The heatmap must therefore represent per-GPU tensor activity intensity, not specific tensor core IDs.

Each heatmap cell is a visual bucket derived from the GPU's aggregate tensor activity. The grid should make high, medium, low, idle, and unavailable states easy to distinguish without implying false hardware precision.

## User Experience

Add a dedicated tensor wall mode toggled with `t`.

In tensor wall mode, each GPU gets a block with:

- GPU index and name
- tensor activity percentage as the primary value
- SM activity, memory pipe activity, and FP32 activity when DCGM provides them
- normal context values: GPU utilization, memory utilization, temperature, and data source
- a large tensor heatmap occupying most of the block

The existing overview and selected-GPU detail flow remains available. Pressing `t` while in tensor wall mode returns to the previous overview/detail flow. Pressing `o` also returns to overview mode explicitly.

## Layout

The tensor wall uses vertical GPU blocks. This scales predictably from a small number of GPUs to larger multi-GPU hosts and keeps each heatmap readable.

The renderer should size the grid from the terminal dimensions and GPU count:

- use larger grids when there is enough vertical and horizontal space
- compact the heatmaps when many GPUs must fit on screen
- preserve at least one readable row per GPU before adding optional detail lines
- keep all labels aligned and avoid wrapping metric text into adjacent columns

If the terminal is too small for the full wall, the view should degrade by reducing heatmap height and secondary metrics before dropping the primary tensor percentage and heatmap.

## Color And No-Color Behavior

Apply the existing Lipgloss style system to the visible UI.

Color levels:

- muted or dim for idle and missing data
- green for active tensor work
- yellow for high activity
- red or pink for saturated activity

The same renderer must respect `--no-color`. In no-color mode, the layout and text stay the same, and the heatmap uses plain block density rather than ANSI color.

## Missing Data

If a GPU lacks DCGM tensor activity, show an explicit unavailable state in that GPU block. The view should continue rendering the other GPUs normally.

If no GPUs have tensor data, the tensor wall should still render a useful explanatory state instead of an empty or misleading heatmap.

## Code Shape

Keep the implementation inside the existing UI package boundaries.

Expected changes:

- add a tensor wall view mode to the UI model
- add key handling so `t` toggles tensor wall mode and `o` exits tensor wall mode
- add renderer functions for GPU tensor blocks and heatmap cells
- wire existing style definitions into overview, detail, and tensor wall rendering where appropriate
- keep DCGM data access through the existing GPU metrics structs

Avoid adding a new rendering framework. The existing string renderer plus Lipgloss is sufficient for this feature.

## Testing

Add focused tests for:

- tensor wall rendering with multiple GPUs
- tensor wall unavailable states when DCGM tensor data is missing
- color output when color is enabled
- no ANSI escapes when `--no-color` is enabled
- key handling for entering and leaving tensor wall mode
- layout stability for narrow or constrained terminal dimensions if the model exposes dimensions to rendering

## Non-Goals

- Do not claim to show physical tensor core identities.
- Do not change CI or release automation.
- Do not require DCGM for the entire application to run.
- Do not replace the existing selected-GPU extended view.
- Do not add mouse interaction or a separate web UI.

## Acceptance Criteria

- Pressing `t` opens a dedicated all-GPU tensor wall.
- Every detected GPU appears in the tensor wall.
- GPUs with tensor data show a large heatmap and tensor percentage.
- GPUs without tensor data show an explicit unavailable state.
- Color improves scanability when enabled.
- `--no-color` remains clean and readable.
- Existing overview and detail behavior remains available.
