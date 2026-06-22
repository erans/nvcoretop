# nvcoretop

`nvcoretop` is a terminal monitor for NVIDIA GPUs.

It exists for local AI inference, training, and development rigs where
`nvidia-smi` is often too coarse: you need to see which card is busy, which
process owns VRAM, whether a GPU is hot or power-limited, whether PCIe/NVLink
traffic is moving, and whether core activity is real DCGM data or an
NVML-derived approximation.

The project is intentionally read-only. It samples driver counters through
NVML, optionally enriches them with DCGM profiling fields, and presents the
same data path either as an interactive TUI or as script-friendly JSONL/CSV.

## What It Provides

- A live terminal UI for 1 to 8+ NVIDIA GPUs.
- Compact per-GPU overview with utilization, memory, temperature, power, and
  core activity.
- Detail view with trends, clocks, throttle reasons, fan speed, process VRAM,
  PCIe throughput, NVLink throughput, and ECC counters.
- Optional DCGM activity fields for SM, Tensor, memory-pipe, and FP32 activity.
- JSONL and CSV export modes for logging, dashboards, and shell pipelines.
- Graceful fallback for unsupported metrics, missing DCGM, permission-limited
  process queries, and small terminals.

## Requirements

- NVIDIA GPU and driver with NVML available on the host.
- Go 1.24 or newer to build from source.
- `CGO_ENABLED=1` and a working C toolchain for normal local builds.
- Optional: NVIDIA DCGM if you want real profiling activity instead of the
  default NVML-derived core visualization.

By default, `nvcoretop` builds without DCGM support and falls back to
representational core activity. To include DCGM support, build with the `dcgm`
tag and run on a host where DCGM can initialize.

## Build

```sh
go build ./cmd/nvcoretop
```

With DCGM support:

```sh
go build -tags dcgm ./cmd/nvcoretop
```

The built binary is written to `./nvcoretop`.

## Usage

Run the TUI:

```sh
./nvcoretop
```

Stream JSONL:

```sh
./nvcoretop --json --interval 1s
```

Stream CSV:

```sh
./nvcoretop --csv --interval 1s --output gpu.csv
```

Capture a bounded sample:

```sh
./nvcoretop --json --count 10
./nvcoretop --csv --duration 5m
```

Export selected fields:

```sh
./nvcoretop --json --fields i,name,util,mem_used,mem_total,temp,power
```

Force DCGM activity:

```sh
./nvcoretop --dcgm
```

If DCGM is not compiled in or cannot initialize, forced DCGM mode exits with an
error. Without `--dcgm`, the tool falls back to NVML-only data.

## TUI Keys

- `up` / `down` or `j` / `k`: select GPU
- `enter` / `tab`: expand or collapse detail
- `s`: cycle sort mode
- `d`: toggle the DCGM activity view
- `p`: pause sampling
- `?`: show help
- `q`: quit

## Export Shape

JSON output is newline-delimited JSON, one snapshot per sample:

```json
{"ts":"2026-06-21T18:40:00Z","source":"NVML","gpus":[{"i":0,"name":"NVIDIA GPU","util":64}]}
```

CSV output is wide: each row is one sample, and per-GPU columns are suffixed by
GPU index. The header is fixed at startup based on the detected GPU count.

Common export fields include:

`i`, `name`, `uuid`, `util`, `mem_util`, `mem_used`, `mem_total`, `temp`,
`power`, `power_limit`, `sm_clock`, `mem_clock`, `fan`, `proc_count`,
`proc_mem`, `pcie_tx`, `pcie_rx`, `nvlink_tx`, `nvlink_rx`, `ecc_sbe`,
`ecc_dbe`, `sm_active`, `tensor_active`, `mem_pipe_active`, `fp32_active`.

Unsupported or unavailable fields are emitted as `null` in JSON and empty
values in CSV.

## Release

Releases are created from version tags that start with `v`.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Pushing the tag runs the release workflow. The workflow verifies the project,
builds a Linux amd64 binary with DCGM support and the tag injected into
`nvcoretop --version`, packages `nvcoretop`, `README.md`, and `LICENSE`, then
attaches the tarball and SHA256 checksum to the GitHub Release.

The release binary can show real SM/Tensor/memory-pipe/FP32 activity when the
host has the DCGM runtime available. Without DCGM at runtime, it falls back to
NVML-only data unless `--dcgm` is forced.

Release candidate tags such as `v0.1.0-rc1` or `v0.1.0-rc.1` are published as
GitHub prereleases.

For a local versioned build:

```sh
go build -trimpath -ldflags "-X main.version=v0.1.0" ./cmd/nvcoretop
```

## Development

Run the default test suite:

```sh
go test ./...
```

Run tests with DCGM-tagged code included:

```sh
go test -tags dcgm ./...
```

Run real-GPU smoke tests on a host with NVIDIA hardware:

```sh
go test -tags gpu ./internal/gpu/nvml ./internal/sampler
```

## License

MIT. See [LICENSE](LICENSE).
