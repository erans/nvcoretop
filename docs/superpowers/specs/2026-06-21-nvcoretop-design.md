# nvcoretop — Design Spec

**Date:** 2026-06-21
**Status:** Approved (brainstorming complete; pending implementation plan)

## 1. Summary

`nvcoretop` is a terminal monitor for NVIDIA GPUs, built for 1–8+ card AI
inference rigs. Its default mode is a live Bubble Tea TUI showing per-card
memory, GPU utilization, temperature, a "cores active" visualization, power,
clocks/throttle reasons, fan, per-process usage, and PCIe/NVLink/ECC — with
sparkline trends. A non-interactive mode streams the same data as JSONL or CSV
at a set interval for logging and analysis.

- **Binary name:** `nvcoretop`
- **Language/stack:** Go + Bubble Tea (Lipgloss for layout/styling, Bubbles for
  viewport/table).
- **GPU APIs:** NVML via `github.com/NVIDIA/go-nvml` (always); DCGM via
  `github.com/NVIDIA/go-dcgm` (optional, for the real cores view).

## 2. Goals & non-goals

### Goals
- Work correctly and stay readable on rigs from 1 to 8+ cards.
- Use the real NVIDIA management API (NVML), not screen-scraped `nvidia-smi`.
- Provide an at-a-glance overview plus on-demand per-card detail.
- Show trends (sparklines), not just instantaneous values.
- Offer a scriptable export mode (JSONL / CSV) at a configurable interval.
- Degrade gracefully: unsupported fields, missing DCGM, small terminals, and
  permission-limited process queries must never crash the tool.

### Non-goals (YAGNI for v1)
- Persistent logging/database, remote/multi-host aggregation.
- Prometheus/metrics endpoint.
- Alerting/notifications.
- Fan/clock/power *control* — `nvcoretop` is strictly read-only.
- Non-NVIDIA GPUs (AMD/Intel).
- Full-screen historical graph view (the ring buffers leave the door open
  architecturally, but the view is not built in v1).

## 3. Key decisions (from brainstorming)

| Decision | Choice |
| --- | --- |
| "Cores active" visualization | **Hybrid**: NVML representational grid as the always-works default; auto-detect DCGM and switch to real SM/Tensor/mem-pipe activity when the daemon is available (or forced with `--dcgm`). |
| Per-card metrics | Memory, GPU util, temperature, cores **plus** power draw+limit, clocks+throttle reasons, fan + per-process usage, and PCIe/NVLink/ECC. |
| Multi-card | First-class. Layout must stay readable at 8+ cards (compact line per card + expandable detail). |
| Export shape | **Streaming, one record per tick containing all GPUs.** JSON = JSONL `{"ts":…,"gpus":[…]}`; CSV = wide, per-card suffixed columns, header fixed at startup card count. |
| History/trends | Rolling in-memory window per card; sparklines for util/temp/power in the detail pane. |
| Language/API | Go + Bubble Tea + official `go-nvml`/`go-dcgm`. Accepted tradeoff: cgo build (`CGO_ENABLED=1` + C compiler); runs anywhere the NVIDIA driver is installed (`libnvidia-ml.so` loaded at runtime). |

## 4. Architecture

The UI and the exporters consume the **same** data path. A single `Sampler`
interface is the seam that keeps GPU access isolated and testable.

```
                ┌─────────────────────────────┐
   ticker ───▶  │  gpu.Sampler (interface)    │ ──▶ gpu.Snapshot
                │   • nvmlSampler (always)    │      (timestamp +
                │   • dcgm enricher (optional)│       []DeviceSample)
                └─────────────────────────────┘
                          │
              ┌───────────┴───────────────┐
              ▼                            ▼
      ui (Bubble Tea)              export (encoder)
   overview + detail +            jsonl / csv writer
   sparklines + cores grid        → stdout / file
```

### Components
- **`gpu`** — core types `Snapshot`, `DeviceSample`, and the `Sampler`
  interface (`Sample() (Snapshot, error)`, `DeviceCount()`, `Close()`). Also
  hosts `FakeSampler` for tests.
- **`gpu/nvml`** — thin adapter mapping `go-nvml` calls to `DeviceSample`. The
  only code that cannot run in CI; smoke-tested on a real GPU behind a build
  tag.
- **`gpu/dcgm`** — optional enricher; if `nv-hostengine` is reachable, fills the
  real activity fields (SM active, Tensor active, mem-pipe active, FP32 pipe).
  Otherwise a no-op.
- **`history`** — fixed-size ring buffers (per device, per metric) that back the
  sparklines.
- **`ui`** — Bubble Tea model/update/view; pure rendering from snapshots +
  history. Sub-renderers: overview list, detail pane, sparkline, cores grid.
- **`export`** — JSONL and CSV encoders.
- **`cmd/nvcoretop`** — flag parsing, mode dispatch, the interval ticker, signal
  handling (graceful Ctrl-C).

## 5. Data model & sampling

- Each tick: `Sample()` returns `Snapshot{ Timestamp time.Time; Devices
  []DeviceSample }`.
- `DeviceSample` carries every selected field. Fields that a card/driver does
  not support are represented as optional (pointer or `Value{ok bool}`) and
  render as `n/a`; they are never assumed present. NVML returns `NOT_SUPPORTED`
  for many fields on consumer cards (e.g. ECC, NVLink), so each field is
  fetched defensively and failures are per-field, not fatal.
- Default interval **1s** for both UI and export, configurable via `--interval`.
- Cores activity: NVML overall GPU-util drives the representational grid by
  default. If DCGM is live, real SM/Tensor/mem-pipe/FP32 values are attached and
  the cores visualization switches to those.

### `DeviceSample` fields (initial set)
- Identity: `Index`, `Name`, `UUID`.
- Memory: `MemUsed`, `MemTotal` (bytes).
- Utilization: `GPUUtil`, `MemUtil` (%).
- Temperature: `TempC`.
- Power: `PowerW`, `PowerLimitW`.
- Clocks: `SMClockMHz`, `MemClockMHz`; `ThrottleReasons` (flag set).
- Fan: `FanPct`.
- Processes: `[]Process{ PID, Name, MemUsed }`.
- Links/errors: `PCIeTxKBps`, `PCIeRxKBps`, `NVLinkTxKBps`, `NVLinkRxKBps`,
  `ECCSingleBit`, `ECCDoubleBit` (all optional).
- DCGM activity (optional): `SMActivePct`, `TensorActivePct`,
  `MemPipeActivePct`, `FP32ActivePct`.

## 6. TUI layout & interaction

### Overview (one compact line per card)
```
 #  NAME        UTIL        MEM            TEMP   PWR        CORES (rep)
 0  RTX 3090   ██████░ 64%  8.1/24.0 GB    71°C   285/350W   ▏███████░░░░
 1  RTX 3090   █░░░░░░ 12%  0.4/24.0 GB    44°C    90/350W   ▏██░░░░░░░░░
 2  RTX 3090   █████████91% 22.6/24.0 GB   83°C△  340/350W   ▏██████████░
```
- Color thresholds for temperature, utilization, and power-near-limit, with a
  `△` throttle marker when a throttle reason is active. `--no-color` honored
  (also auto-disabled when output is not a TTY).

### Detail pane (selected card, toggled)
- Sparklines for util / temp / power over the rolling window.
- Full clocks + throttle reasons, fan, PCIe/NVLink/ECC.
- Per-process table: PID, name, VRAM used.
- When DCGM is active, the cores section shows real SM/Tensor/mem-pipe/FP32
  bars instead of the representational grid.

### Keys
`↑/↓` or `j/k` select · `enter`/`tab` expand/collapse detail · `s` cycle sort
(index/util/temp/mem/power) · `d` toggle DCGM activity view · `p` pause · `?`
help · `q` quit. Footer shows interval, current sort, and data source
(`NVML` or `NVML+DCGM`).

### Degraded layout
If the terminal is too small for the overview columns, fall back to a
single-column, abbreviated layout.

## 7. Export mode (non-interactive)

- `--json` → JSONL, one object per tick:
  `{"ts":"2026-06-21T18:40:00Z","gpus":[{"i":0,"util":64,"mem_used":…,…}, …]}`.
- `--csv` → wide format, one row per tick; columns suffixed per card
  (`util_gpu0,temp_gpu0,…,util_gpu1,…`). Header written once; the card set is
  fixed at startup.
- Streams to stdout (pipe/redirect friendly). `--output FILE` writes a file
  (`-` = stdout). `--duration` / `--count` auto-stop; otherwise runs until
  Ctrl-C. Optional `--fields` subsets columns. DCGM activity fields are included
  automatically when available.

## 8. CLI surface

```
nvcoretop                       # interactive TUI (default)
nvcoretop --json                # stream JSONL to stdout
nvcoretop --csv                 # stream wide CSV to stdout
  --interval DURATION           # sample interval (default 1s)
  --output FILE                 # export destination ('-' = stdout, default)
  --duration DURATION           # export: auto-stop after this long
  --count N                     # export: auto-stop after N ticks
  --fields LIST                 # export: subset of columns
  --dcgm                        # force real DCGM activity (error if absent)
  --no-color                    # disable colored output
  --version / --help
```
`--json` and `--csv` are mutually exclusive; supplying either selects export
mode, otherwise the TUI runs.

## 9. Error handling

- NVML init fails (no driver/lib) → clear message to stderr, non-zero exit.
- DCGM unavailable → silently fall back to representational mode, with a
  one-line notice (UI footer / stderr). If `--dcgm` was explicitly requested and
  DCGM is unreachable → error out non-zero.
- Unsupported per-field values → render `n/a`; never fatal.
- Per-process query `ACCESS_DENIED` (permissions) → show what is available /
  note the limitation; do not crash.
- Terminal too small → degraded single-column layout.

## 10. Testing strategy

- Everything except the NVML adapter is tested via `FakeSampler` (scripted
  snapshots) plus an injectable clock for determinism.
- **Export**: golden-file tests for JSONL and CSV (deterministic given a fixed
  snapshot + fixed timestamp).
- **History**: ring-buffer unit tests (wrap-around, partial fill, fixed size).
- **UI**: model/update unit tests; `teatest` snapshot tests of rendered views.
- **NVML adapter**: thin mapping layer, smoke-tested on a real GPU behind a
  build tag; excluded from CI.

## 11. Project layout

```
nvcoretop/
  cmd/nvcoretop/main.go      flags, mode dispatch, ticker, signals
  internal/gpu/              Snapshot, DeviceSample, Sampler iface, FakeSampler
  internal/gpu/nvml/         go-nvml adapter (build-tagged smoke test)
  internal/gpu/dcgm/         go-dcgm enricher
  internal/history/          ring buffers
  internal/ui/               Bubble Tea model, views, sparkline, cores grid
  internal/export/           jsonl + csv encoders
  go.mod
```

## 12. Open considerations for the implementation plan
- Exact color thresholds per metric (sensible defaults; possibly tuned for
  common cards). Defaults documented in `ui`.
- Whether `--fields` ships in v1 or is deferred (low risk to defer).
- DCGM embedded mode vs. connecting to an existing `nv-hostengine` (prefer
  connecting if present, fall back to embedded if permitted).
