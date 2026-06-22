# nvcoretop NVIDIA Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fake sampling with real NVML sampling and optional DCGM activity enrichment while keeping unsupported fields and missing permissions non-fatal.

**Architecture:** `internal/gpu/nvml` implements the required `gpu.Sampler` and owns all NVML calls. `internal/gpu/dcgm` implements an optional `gpu.Enricher`; the default build provides a no-op enricher, and `-tags dcgm` enables the real go-dcgm implementation.

**Tech Stack:** Go 1.26 with CGO enabled, `github.com/NVIDIA/go-nvml@v0.13.2-0`, `github.com/NVIDIA/go-dcgm@v0.0.0-20260603204728-453d82102783`, NVML runtime library from the NVIDIA driver, optional DCGM hostengine or embedded DCGM.

---

## File Structure

- Modify: `go.mod` / `go.sum` - add NVIDIA dependencies.
- Modify: `internal/gpu/types.go` - add `Enricher` interface for optional DCGM data.
- Create: `internal/gpu/nvml/errors.go` - NVML return helpers and fatal init error formatting.
- Create: `internal/gpu/nvml/fields.go` - per-device field mapping, throttle bit mapping, process name lookup, and NVLink delta helpers.
- Create: `internal/gpu/nvml/fields_test.go` - CI-safe mapping tests using `github.com/NVIDIA/go-nvml/pkg/nvml/mock`.
- Create: `internal/gpu/nvml/sampler.go` - NVML init, device handle discovery, sample loop, DCGM enrichment, close.
- Create: `internal/gpu/nvml/sampler_gpu_test.go` - real GPU smoke test behind `//go:build gpu`.
- Create: `internal/gpu/dcgm/enricher_stub.go` - default no-op DCGM implementation for builds without `-tags dcgm`.
- Create: `internal/gpu/dcgm/enricher_stub_test.go` - tests forced DCGM errors in default builds.
- Create: `internal/gpu/dcgm/enricher_real.go` - real DCGM implementation behind `//go:build dcgm`.
- Create: `internal/gpu/dcgm/enricher_dcgm_test.go` - real DCGM smoke test behind `//go:build dcgm && gpu`.
- Create: `internal/sampler/factory.go` - creates NVML sampler plus optional DCGM enricher.
- Create: `internal/sampler/factory_gpu_test.go` - real factory smoke test behind `//go:build gpu`.
- Modify: `cmd/nvcoretop/main.go` - use the real sampler factory for export and TUI modes.

## References Checked

- `go-nvml` exposes `nvml.Init`, `nvml.DeviceGetHandleByIndex`, `Device.GetMemoryInfo`, `Device.GetUtilizationRates`, `Device.GetPowerUsage`, `Device.GetClockInfo`, `Device.GetComputeRunningProcesses`, and related methods in `github.com/NVIDIA/go-nvml/pkg/nvml`.
- `go-dcgm` exposes `dcgm.Init`, modes `Standalone` and `Embedded`, `FieldGroupCreate`, `WatchFields`, `UpdateAllFields`, and `GetLatestValuesForFields` in `github.com/NVIDIA/go-dcgm/pkg/dcgm`.

## Prerequisites

Complete these plans first:

- `docs/superpowers/plans/2026-06-22-nvcoretop-core.md`
- `docs/superpowers/plans/2026-06-22-nvcoretop-export-cli.md`
- `docs/superpowers/plans/2026-06-22-nvcoretop-tui.md`

## Decisions Locked In

- Normal builds do not require DCGM. Real DCGM support is compiled with `go build -tags dcgm`.
- `--dcgm` without a DCGM-enabled build returns a clear non-zero error.
- Unsupported NVML fields map to missing `gpu.Optional` values and never fail the whole sample.
- Per-process `ERROR_NO_PERMISSION` sets `DeviceSample.ProcessLimited = true`.
- PCIe throughput comes from NVML `GetPcieThroughput` and is already KB/s.
- NVLink throughput is computed from utilization counter deltas; first sample has no NVLink throughput values.

### Task 1: Add NVIDIA Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add pinned modules**

Run:

```bash
go get github.com/NVIDIA/go-nvml@v0.13.2-0
go get github.com/NVIDIA/go-dcgm@v0.0.0-20260603204728-453d82102783
go mod tidy
```

Expected: commands succeed with `CGO_ENABLED=1`.

- [ ] **Step 2: Verify existing tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add nvidia gpu dependencies"
```

### Task 2: Add Enricher Interface

**Files:**
- Modify: `internal/gpu/types.go`

- [ ] **Step 1: Add interface below `Sampler`**

In `internal/gpu/types.go`, append:

```go
type Enricher interface {
	Enrich(context.Context, Snapshot) (Snapshot, error)
	Active() bool
	Notice() string
	Close() error
}
```

- [ ] **Step 2: Run core tests**

Run: `go test ./internal/gpu -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/gpu/types.go
git commit -m "feat: add gpu snapshot enricher contract"
```

### Task 3: Implement NVML Mapping Helpers

**Files:**
- Create: `internal/gpu/nvml/fields_test.go`
- Create: `internal/gpu/nvml/errors.go`
- Create: `internal/gpu/nvml/fields.go`

- [ ] **Step 1: Write mapping tests**

Create `internal/gpu/nvml/fields_test.go` with:

```go
package nvml

import (
	"testing"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock"
	"nvcoretop/internal/gpu"
)

func TestMapThrottleReasons(t *testing.T) {
	reasons := mapThrottleReasons(
		nvidia.ClocksThrottleReasonGpuIdle |
			nvidia.ClocksThrottleReasonSwPowerCap |
			nvidia.ClocksThrottleReasonHwThermalSlowdown,
	)
	if !reasons.GPUIdle || !reasons.SWPowerCap || !reasons.HWThermal {
		t.Fatalf("reasons = %#v", reasons)
	}
}

func TestSampleDeviceMapsSupportedFields(t *testing.T) {
	device := &mock.Device{
		GetNameFunc: func() (string, nvidia.Return) { return "RTX 3090", nvidia.SUCCESS },
		GetUUIDFunc: func() (string, nvidia.Return) { return "GPU-test", nvidia.SUCCESS },
		GetMemoryInfoFunc: func() (nvidia.Memory, nvidia.Return) {
			return nvidia.Memory{Used: 8, Total: 24}, nvidia.SUCCESS
		},
		GetUtilizationRatesFunc: func() (nvidia.Utilization, nvidia.Return) {
			return nvidia.Utilization{Gpu: 64, Memory: 12}, nvidia.SUCCESS
		},
		GetTemperatureFunc: func(nvidia.TemperatureSensors) (uint32, nvidia.Return) {
			return 71, nvidia.SUCCESS
		},
		GetPowerUsageFunc: func() (uint32, nvidia.Return) { return 285000, nvidia.SUCCESS },
		GetPowerManagementLimitFunc: func() (uint32, nvidia.Return) { return 350000, nvidia.SUCCESS },
		GetClockInfoFunc: func(clock nvidia.ClockType) (uint32, nvidia.Return) {
			if clock == nvidia.CLOCK_SM {
				return 1800, nvidia.SUCCESS
			}
			return 9500, nvidia.SUCCESS
		},
		GetCurrentClocksThrottleReasonsFunc: func() (uint64, nvidia.Return) {
			return nvidia.ClocksThrottleReasonSwPowerCap, nvidia.SUCCESS
		},
		GetFanSpeedFunc: func() (uint32, nvidia.Return) { return 55, nvidia.SUCCESS },
		GetComputeRunningProcessesFunc: func() ([]nvidia.ProcessInfo, nvidia.Return) {
			return []nvidia.ProcessInfo{{Pid: 123, UsedGpuMemory: 4096}}, nvidia.SUCCESS
		},
		GetPcieThroughputFunc: func(counter nvidia.PcieUtilCounter) (uint32, nvidia.Return) {
			if counter == nvidia.PCIE_UTIL_TX_BYTES {
				return 111, nvidia.SUCCESS
			}
			return 222, nvidia.SUCCESS
		},
		GetTotalEccErrorsFunc: func(kind nvidia.MemoryErrorType, counter nvidia.EccCounterType) (uint64, nvidia.Return) {
			if kind == nvidia.MEMORY_ERROR_TYPE_CORRECTED {
				return 1, nvidia.SUCCESS
			}
			return 2, nvidia.SUCCESS
		},
		GetNvLinkStateFunc: func(int) (nvidia.EnableState, nvidia.Return) {
			return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
		},
	}

	got := sampleDevice(0, device, func(uint32) string { return "python" }, nil)
	if got.Name != "RTX 3090" || got.UUID != "GPU-test" {
		t.Fatalf("identity = %#v", got)
	}
	if got.GPUUtil.Value != 64 || got.PowerW.Value != 285 || got.Processes[0].Name != "python" {
		t.Fatalf("sample = %#v", got)
	}
	if !got.ThrottleReasons.SWPowerCap {
		t.Fatalf("missing throttle reason: %#v", got.ThrottleReasons)
	}
}

func TestSampleDeviceMarksPermissionLimitedProcesses(t *testing.T) {
	device := minimalMockDevice()
	device.GetComputeRunningProcessesFunc = func() ([]nvidia.ProcessInfo, nvidia.Return) {
		return nil, nvidia.ERROR_NO_PERMISSION
	}

	got := sampleDevice(0, device, func(uint32) string { return "" }, nil)
	if !got.ProcessLimited {
		t.Fatalf("ProcessLimited = false, want true")
	}
}

func TestUnsupportedFieldsStayMissing(t *testing.T) {
	device := minimalMockDevice()
	device.GetTemperatureFunc = func(nvidia.TemperatureSensors) (uint32, nvidia.Return) {
		return 0, nvidia.ERROR_NOT_SUPPORTED
	}

	got := sampleDevice(0, device, func(uint32) string { return "" }, nil)
	if got.TempC.OK {
		t.Fatalf("TempC = %#v, want missing", got.TempC)
	}
}

func minimalMockDevice() *mock.Device {
	return &mock.Device{
		GetNameFunc: func() (string, nvidia.Return) { return "GPU", nvidia.SUCCESS },
		GetUUIDFunc: func() (string, nvidia.Return) { return "uuid", nvidia.SUCCESS },
		GetMemoryInfoFunc: func() (nvidia.Memory, nvidia.Return) { return nvidia.Memory{}, nvidia.SUCCESS },
		GetUtilizationRatesFunc: func() (nvidia.Utilization, nvidia.Return) { return nvidia.Utilization{}, nvidia.SUCCESS },
		GetTemperatureFunc: func(nvidia.TemperatureSensors) (uint32, nvidia.Return) { return 0, nvidia.SUCCESS },
		GetPowerUsageFunc: func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetPowerManagementLimitFunc: func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetClockInfoFunc: func(nvidia.ClockType) (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetCurrentClocksThrottleReasonsFunc: func() (uint64, nvidia.Return) { return 0, nvidia.SUCCESS },
		GetFanSpeedFunc: func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetComputeRunningProcessesFunc: func() ([]nvidia.ProcessInfo, nvidia.Return) { return nil, nvidia.SUCCESS },
		GetPcieThroughputFunc: func(nvidia.PcieUtilCounter) (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetTotalEccErrorsFunc: func(nvidia.MemoryErrorType, nvidia.EccCounterType) (uint64, nvidia.Return) {
			return 0, nvidia.ERROR_NOT_SUPPORTED
		},
		GetNvLinkStateFunc: func(int) (nvidia.EnableState, nvidia.Return) {
			return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
		},
	}
}

var _ gpu.DeviceSample
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/gpu/nvml -run 'TestMapThrottleReasons|TestSampleDevice|TestUnsupportedFields' -v`

Expected: FAIL with undefined `mapThrottleReasons`, `sampleDevice`, and `minimalMockDevice` helper dependencies in implementation.

- [ ] **Step 3: Implement errors**

Create `internal/gpu/nvml/errors.go` with:

```go
package nvml

import (
	"fmt"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
)

func ok(ret nvidia.Return) bool {
	return ret == nvidia.SUCCESS
}

func errString(prefix string, ret nvidia.Return) error {
	return fmt.Errorf("%s: %s", prefix, nvidia.ErrorString(ret))
}
```

- [ ] **Step 4: Implement field mapping**

Create `internal/gpu/nvml/fields.go` with:

```go
package nvml

import (
	"os"
	"strconv"
	"strings"
	"time"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"nvcoretop/internal/gpu"
)

const maxNVLinks = 12

type nvlinkTotals struct {
	at time.Time
	tx uint64
	rx uint64
}

func sampleDevice(index int, device nvidia.Device, processName func(uint32) string, previous *nvlinkTotals) gpu.DeviceSample {
	sample := gpu.DeviceSample{Index: index}

	if name, ret := device.GetName(); ok(ret) {
		sample.Name = name
	}
	if uuid, ret := device.GetUUID(); ok(ret) {
		sample.UUID = uuid
	}
	if mem, ret := device.GetMemoryInfo(); ok(ret) {
		sample.MemUsed = mem.Used
		sample.MemTotal = mem.Total
	}
	if util, ret := device.GetUtilizationRates(); ok(ret) {
		sample.GPUUtil = gpu.Some(util.Gpu)
		sample.MemUtil = gpu.Some(util.Memory)
	}
	if temp, ret := device.GetTemperature(nvidia.TEMPERATURE_GPU); ok(ret) {
		sample.TempC = gpu.Some(temp)
	}
	if milliWatts, ret := device.GetPowerUsage(); ok(ret) {
		sample.PowerW = gpu.Some(float64(milliWatts) / 1000)
	}
	if milliWatts, ret := device.GetPowerManagementLimit(); ok(ret) {
		sample.PowerLimitW = gpu.Some(float64(milliWatts) / 1000)
	}
	if clock, ret := device.GetClockInfo(nvidia.CLOCK_SM); ok(ret) {
		sample.SMClockMHz = gpu.Some(clock)
	}
	if clock, ret := device.GetClockInfo(nvidia.CLOCK_MEM); ok(ret) {
		sample.MemClockMHz = gpu.Some(clock)
	}
	if reasons, ret := device.GetCurrentClocksThrottleReasons(); ok(ret) {
		sample.ThrottleReasons = mapThrottleReasons(reasons)
	}
	if fan, ret := device.GetFanSpeed(); ok(ret) {
		sample.FanPct = gpu.Some(fan)
	}
	if processes, ret := device.GetComputeRunningProcesses(); ok(ret) {
		sample.Processes = mapProcesses(processes, processName)
	} else if ret == nvidia.ERROR_NO_PERMISSION {
		sample.ProcessLimited = true
	}
	if value, ret := device.GetPcieThroughput(nvidia.PCIE_UTIL_TX_BYTES); ok(ret) {
		sample.PCIeTxKBps = gpu.Some(uint64(value))
	}
	if value, ret := device.GetPcieThroughput(nvidia.PCIE_UTIL_RX_BYTES); ok(ret) {
		sample.PCIeRxKBps = gpu.Some(uint64(value))
	}
	if value, ret := device.GetTotalEccErrors(nvidia.MEMORY_ERROR_TYPE_CORRECTED, nvidia.VOLATILE_ECC); ok(ret) {
		sample.ECCSingleBit = gpu.Some(value)
	}
	if value, ret := device.GetTotalEccErrors(nvidia.MEMORY_ERROR_TYPE_UNCORRECTED, nvidia.VOLATILE_ECC); ok(ret) {
		sample.ECCDoubleBit = gpu.Some(value)
	}

	return sample
}

func mapProcesses(processes []nvidia.ProcessInfo, processName func(uint32) string) []gpu.Process {
	out := make([]gpu.Process, 0, len(processes))
	for _, process := range processes {
		out = append(out, gpu.Process{
			PID:     process.Pid,
			Name:    processName(process.Pid),
			MemUsed: process.UsedGpuMemory,
		})
	}
	return out
}

func processNameFromProc(pid uint32) string {
	data, err := os.ReadFile("/proc/" + strconv.FormatUint(uint64(pid), 10) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func mapThrottleReasons(bits uint64) gpu.ThrottleReasons {
	return gpu.ThrottleReasons{
		GPUIdle:             bits&nvidia.ClocksThrottleReasonGpuIdle != 0,
		ApplicationsClocks:  bits&nvidia.ClocksThrottleReasonApplicationsClocksSetting != 0,
		SWPowerCap:          bits&nvidia.ClocksThrottleReasonSwPowerCap != 0,
		HWSlowdown:          bits&nvidia.ClocksThrottleReasonHwSlowdown != 0,
		SyncBoost:           bits&nvidia.ClocksThrottleReasonSyncBoost != 0,
		SWThermal:           bits&nvidia.ClocksThrottleReasonSwThermalSlowdown != 0,
		HWThermal:           bits&nvidia.ClocksThrottleReasonHwThermalSlowdown != 0,
		HWPowerBrake:        bits&nvidia.ClocksThrottleReasonHwPowerBrakeSlowdown != 0,
	}
}
```

- [ ] **Step 5: Run mapping tests**

Run: `go test ./internal/gpu/nvml -run 'TestMapThrottleReasons|TestSampleDevice|TestUnsupportedFields' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gpu/nvml/errors.go internal/gpu/nvml/fields.go internal/gpu/nvml/fields_test.go
git commit -m "feat: map nvml device fields"
```

### Task 4: Implement NVML Sampler

**Files:**
- Create: `internal/gpu/nvml/sampler.go`
- Create: `internal/gpu/nvml/sampler_gpu_test.go`

- [ ] **Step 1: Implement sampler**

Create `internal/gpu/nvml/sampler.go` with:

```go
package nvml

import (
	"context"
	"sync"
	"time"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"nvcoretop/internal/gpu"
)

type Options struct {
	Now      func() time.Time
	Enricher gpu.Enricher
}

type Sampler struct {
	mu       sync.Mutex
	devices  []nvidia.Device
	now      func() time.Time
	enricher gpu.Enricher
	closed   bool
}

func New(options Options) (*Sampler, error) {
	if ret := nvidia.Init(); !ok(ret) {
		return nil, errString("NVML init failed", ret)
	}
	count, ret := nvidia.DeviceGetCount()
	if !ok(ret) {
		_ = nvidia.Shutdown()
		return nil, errString("NVML device count failed", ret)
	}
	devices := make([]nvidia.Device, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvidia.DeviceGetHandleByIndex(i)
		if !ok(ret) {
			_ = nvidia.Shutdown()
			return nil, errString("NVML device handle failed", ret)
		}
		devices = append(devices, device)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Sampler{
		devices:  devices,
		now:      now,
		enricher: options.Enricher,
	}, nil
}

func (s *Sampler) Sample(ctx context.Context) (gpu.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return gpu.Snapshot{}, gpu.ErrSamplerClosed
	}

	snapshot := gpu.Snapshot{
		Timestamp: s.now().UTC(),
		Source:    gpu.SourceNVML,
		Devices:   make([]gpu.DeviceSample, 0, len(s.devices)),
	}
	for index, device := range s.devices {
		snapshot.Devices = append(snapshot.Devices, sampleDevice(index, device, processNameFromProc, nil))
	}

	if s.enricher != nil {
		enriched, err := s.enricher.Enrich(ctx, snapshot)
		if err != nil {
			return snapshot, err
		}
		return enriched, nil
	}
	return snapshot, nil
}

func (s *Sampler) DeviceCount() int {
	return len(s.devices)
}

func (s *Sampler) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.enricher != nil {
		_ = s.enricher.Close()
	}
	ret := nvidia.Shutdown()
	if !ok(ret) {
		return errString("NVML shutdown failed", ret)
	}
	return nil
}
```

- [ ] **Step 2: Add real GPU smoke test**

Create `internal/gpu/nvml/sampler_gpu_test.go` with:

```go
//go:build gpu

package nvml

import (
	"context"
	"testing"
)

func TestSamplerSmokeOnRealGPU(t *testing.T) {
	sampler, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer sampler.Close()

	snapshot, err := sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample error = %v", err)
	}
	if len(snapshot.Devices) == 0 {
		t.Fatalf("no GPUs found")
	}
	if snapshot.Devices[0].Name == "" {
		t.Fatalf("first GPU name empty: %#v", snapshot.Devices[0])
	}
}
```

- [ ] **Step 3: Run CI-safe tests**

Run: `go test ./internal/gpu/nvml -v`

Expected: PASS without requiring a real GPU.

- [ ] **Step 4: Run optional real GPU smoke test on a GPU host**

Run: `go test -tags gpu ./internal/gpu/nvml -run TestSamplerSmokeOnRealGPU -v`

Expected on a GPU host with NVIDIA driver: PASS. Expected without NVIDIA driver: FAIL with a clear NVML init message.

- [ ] **Step 5: Commit**

```bash
git add internal/gpu/nvml/sampler.go internal/gpu/nvml/sampler_gpu_test.go
git commit -m "feat: add nvml sampler"
```

### Task 5: Add NVLink Throughput Deltas

**Files:**
- Modify: `internal/gpu/nvml/fields_test.go`
- Modify: `internal/gpu/nvml/fields.go`
- Modify: `internal/gpu/nvml/sampler.go`

- [ ] **Step 1: Write NVLink delta test**

Append to `internal/gpu/nvml/fields_test.go`:

```go
func TestApplyNVLinkDelta(t *testing.T) {
	previous := nvlinkTotals{at: time.Unix(10, 0), tx: 1024, rx: 2048}
	current := nvlinkTotals{at: time.Unix(12, 0), tx: 3072, rx: 6144}
	sample := gpu.DeviceSample{Index: 0}

	applyNVLinkDelta(&sample, previous, current)

	if !sample.NVLinkTxKBps.OK || sample.NVLinkTxKBps.Value != 1 {
		t.Fatalf("NVLinkTxKBps = %#v, want 1", sample.NVLinkTxKBps)
	}
	if !sample.NVLinkRxKBps.OK || sample.NVLinkRxKBps.Value != 2 {
		t.Fatalf("NVLinkRxKBps = %#v, want 2", sample.NVLinkRxKBps)
	}
}
```

Also add `time` to the imports in `fields_test.go`.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/gpu/nvml -run TestApplyNVLinkDelta -v`

Expected: FAIL with undefined `applyNVLinkDelta`.

- [ ] **Step 3: Add NVLink helpers**

Append to `internal/gpu/nvml/fields.go`:

```go
func readNVLinkTotals(device nvidia.Device, at time.Time) (nvlinkTotals, bool) {
	totals := nvlinkTotals{at: at}
	found := false
	for link := 0; link < maxNVLinks; link++ {
		state, ret := device.GetNvLinkState(link)
		if !ok(ret) || state != nvidia.FEATURE_ENABLED {
			continue
		}
		rx, tx, ret := device.GetNvLinkUtilizationCounter(link, 0)
		if !ok(ret) {
			continue
		}
		totals.rx += rx
		totals.tx += tx
		found = true
	}
	return totals, found
}

func applyNVLinkDelta(sample *gpu.DeviceSample, previous, current nvlinkTotals) {
	elapsed := current.at.Sub(previous.at).Seconds()
	if elapsed <= 0 || current.tx < previous.tx || current.rx < previous.rx {
		return
	}
	sample.NVLinkTxKBps = gpu.Some(uint64((float64(current.tx-previous.tx) / 1024) / elapsed))
	sample.NVLinkRxKBps = gpu.Some(uint64((float64(current.rx-previous.rx) / 1024) / elapsed))
}
```

- [ ] **Step 4: Store previous NVLink counters in sampler**

In `internal/gpu/nvml/sampler.go`, add a field to `Sampler`:

```go
	lastNVLink map[int]nvlinkTotals
```

Initialize it in `New`:

```go
	lastNVLink: make(map[int]nvlinkTotals),
```

Replace the device loop in `Sample` with:

```go
	for index, device := range s.devices {
		deviceSample := sampleDevice(index, device, processNameFromProc, nil)
		if totals, found := readNVLinkTotals(device, snapshot.Timestamp); found {
			if previous, ok := s.lastNVLink[index]; ok {
				applyNVLinkDelta(&deviceSample, previous, totals)
			}
			s.lastNVLink[index] = totals
		}
		snapshot.Devices = append(snapshot.Devices, deviceSample)
	}
```

- [ ] **Step 5: Run NVML tests**

Run: `go test ./internal/gpu/nvml -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gpu/nvml/fields.go internal/gpu/nvml/fields_test.go internal/gpu/nvml/sampler.go
git commit -m "feat: compute nvlink throughput"
```

### Task 6: Add Default DCGM Stub

**Files:**
- Create: `internal/gpu/dcgm/enricher_stub_test.go`
- Create: `internal/gpu/dcgm/enricher_stub.go`

- [ ] **Step 1: Write stub tests**

Create `internal/gpu/dcgm/enricher_stub_test.go` with:

```go
//go:build !dcgm

package dcgm

import (
	"context"
	"strings"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestNewStubFallsBackWhenNotForced(t *testing.T) {
	enricher, err := New(false, 1)
	if err != nil {
		t.Fatalf("New(false) error = %v", err)
	}
	if enricher.Active() {
		t.Fatalf("stub Active() = true, want false")
	}
	snapshot := gpu.Snapshot{Devices: []gpu.DeviceSample{{Index: 0}}}
	got, err := enricher.Enrich(context.Background(), snapshot)
	if err != nil || len(got.Devices) != 1 {
		t.Fatalf("Enrich = %#v, %v", got, err)
	}
}

func TestNewStubErrorsWhenForced(t *testing.T) {
	_, err := New(true, 1)
	if err == nil || !strings.Contains(err.Error(), "not compiled") {
		t.Fatalf("New(true) error = %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/gpu/dcgm -run TestNewStub -v`

Expected: FAIL with undefined `New`.

- [ ] **Step 3: Implement stub**

Create `internal/gpu/dcgm/enricher_stub.go` with:

```go
//go:build !dcgm

package dcgm

import (
	"context"
	"errors"

	"nvcoretop/internal/gpu"
)

var ErrNotCompiled = errors.New("DCGM support not compiled; rebuild with -tags dcgm")

type noop struct {
	notice string
}

func New(force bool, deviceCount int) (gpu.Enricher, error) {
	if force {
		return nil, ErrNotCompiled
	}
	return noop{notice: "DCGM unavailable; using NVML representational cores"}, nil
}

func (n noop) Enrich(_ context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	return snapshot, nil
}

func (n noop) Active() bool {
	return false
}

func (n noop) Notice() string {
	return n.notice
}

func (n noop) Close() error {
	return nil
}
```

- [ ] **Step 4: Run stub tests**

Run: `go test ./internal/gpu/dcgm -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gpu/dcgm/enricher_stub.go internal/gpu/dcgm/enricher_stub_test.go
git commit -m "feat: add dcgm fallback enricher"
```

### Task 7: Add Real DCGM Enricher Behind Build Tag

**Files:**
- Create: `internal/gpu/dcgm/enricher_real.go`
- Create: `internal/gpu/dcgm/enricher_dcgm_test.go`

- [ ] **Step 1: Implement real enricher**

Create `internal/gpu/dcgm/enricher_real.go` with:

```go
//go:build dcgm

package dcgm

import (
	"context"
	"fmt"

	nvidia "github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"nvcoretop/internal/gpu"
)

type noop struct {
	notice string
}

func (n noop) Enrich(_ context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	return snapshot, nil
}

func (n noop) Active() bool {
	return false
}

func (n noop) Notice() string {
	return n.notice
}

func (n noop) Close() error {
	return nil
}

var activityFields = []nvidia.Short{
	nvidia.DCGM_FI_PROF_SM_ACTIVE,
	nvidia.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	nvidia.DCGM_FI_PROF_DRAM_ACTIVE,
	nvidia.DCGM_FI_PROF_PIPE_FP32_ACTIVE,
}

type Client struct {
	cleanup    func()
	fieldGroup nvidia.FieldHandle
	groups     []nvidia.GroupHandle
	mode       string
}

func New(force bool, deviceCount int) (gpu.Enricher, error) {
	cleanup, err := nvidia.Init(nvidia.Standalone)
	mode := "standalone"
	if err != nil {
		cleanup, err = nvidia.Init(nvidia.Embedded)
		mode = "embedded"
	}
	if err != nil {
		if force {
			return nil, fmt.Errorf("DCGM unavailable: %w", err)
		}
		return noop{notice: "DCGM unavailable; using NVML representational cores"}, nil
	}

	fieldGroup, err := nvidia.FieldGroupCreate("nvcoretop-prof", activityFields)
	if err != nil {
		cleanup()
		if force {
			return nil, err
		}
		return noop{notice: "DCGM fields unavailable; using NVML representational cores"}, nil
	}

	client := &Client{cleanup: cleanup, fieldGroup: fieldGroup, mode: mode}
	for i := 0; i < deviceCount; i++ {
		group, err := nvidia.WatchFields(uint(i), fieldGroup, fmt.Sprintf("nvcoretop-gpu-%d", i))
		if err != nil {
			_ = client.Close()
			if force {
				return nil, err
			}
			return noop{notice: "DCGM watch unavailable; using NVML representational cores"}, nil
		}
		client.groups = append(client.groups, group)
	}
	return client, nil
}

func (c *Client) Enrich(ctx context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	select {
	case <-ctx.Done():
		return snapshot, ctx.Err()
	default:
	}

	if err := nvidia.UpdateAllFields(); err != nil {
		return snapshot, err
	}

	for i := range snapshot.Devices {
		values, err := nvidia.GetLatestValuesForFields(uint(snapshot.Devices[i].Index), activityFields)
		if err != nil {
			continue
		}
		for _, value := range values {
			if value.Status != 0 {
				continue
			}
			percent := normalizePercent(value.Float64())
			switch value.FieldID {
			case nvidia.DCGM_FI_PROF_SM_ACTIVE:
				snapshot.Devices[i].SMActivePct = gpu.Some(percent)
			case nvidia.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
				snapshot.Devices[i].TensorActivePct = gpu.Some(percent)
			case nvidia.DCGM_FI_PROF_DRAM_ACTIVE:
				snapshot.Devices[i].MemPipeActivePct = gpu.Some(percent)
			case nvidia.DCGM_FI_PROF_PIPE_FP32_ACTIVE:
				snapshot.Devices[i].FP32ActivePct = gpu.Some(percent)
			}
		}
	}
	snapshot.Source = gpu.SourceNVMLDCGM
	return snapshot, nil
}

func (c *Client) Active() bool {
	return true
}

func (c *Client) Notice() string {
	return "DCGM active (" + c.mode + ")"
}

func (c *Client) Close() error {
	for _, group := range c.groups {
		_ = nvidia.DestroyGroup(group)
	}
	if c.fieldGroup.GetHandle() != 0 {
		_ = nvidia.FieldGroupDestroy(c.fieldGroup)
	}
	if c.cleanup != nil {
		c.cleanup()
	}
	return nil
}

func normalizePercent(value float64) float64 {
	if value <= 1 {
		return value * 100
	}
	return value
}
```

- [ ] **Step 2: Add real DCGM smoke test**

Create `internal/gpu/dcgm/enricher_dcgm_test.go` with:

```go
//go:build dcgm && gpu

package dcgm

import (
	"context"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestRealDCGMEnricherSmoke(t *testing.T) {
	enricher, err := New(false, 1)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer enricher.Close()
	if !enricher.Active() {
		t.Skip("DCGM inactive on this host")
	}
	_, err = enricher.Enrich(context.Background(), gpu.Snapshot{
		Devices: []gpu.DeviceSample{{Index: 0}},
	})
	if err != nil {
		t.Fatalf("Enrich error = %v", err)
	}
}
```

- [ ] **Step 3: Run default-build tests**

Run: `go test ./internal/gpu/dcgm -v`

Expected: PASS using the stub implementation.

- [ ] **Step 4: Run optional DCGM build test**

Run: `go test -tags dcgm ./internal/gpu/dcgm -run TestNewStub -v`

Expected: package builds with the real implementation and reports no tests to run because the stub test file has `//go:build !dcgm`.

- [ ] **Step 5: Run optional GPU and DCGM smoke test**

Run: `go test -tags 'dcgm gpu' ./internal/gpu/dcgm -run TestRealDCGMEnricherSmoke -v`

Expected on a DCGM-capable GPU host: PASS or SKIP if DCGM is inactive.

- [ ] **Step 6: Commit**

```bash
git add internal/gpu/dcgm/enricher_real.go internal/gpu/dcgm/enricher_dcgm_test.go
git commit -m "feat: add optional dcgm activity enricher"
```

### Task 8: Add Real Sampler Factory and CLI Wiring

**Files:**
- Create: `internal/sampler/factory.go`
- Create: `internal/sampler/factory_gpu_test.go`
- Modify: `cmd/nvcoretop/main.go`

- [ ] **Step 1: Implement factory**

Create `internal/sampler/factory.go` with:

```go
package sampler

import (
	"time"

	"nvcoretop/internal/gpu"
	"nvcoretop/internal/gpu/dcgm"
	"nvcoretop/internal/gpu/nvml"
)

type Options struct {
	ForceDCGM bool
	Now       func() time.Time
}

type Result struct {
	Sampler gpu.Sampler
	Notice  string
}

func New(options Options) (Result, error) {
	base, err := nvml.New(nvml.Options{Now: options.Now})
	if err != nil {
		return Result{}, err
	}

	enricher, err := dcgm.New(options.ForceDCGM, base.DeviceCount())
	if err != nil {
		_ = base.Close()
		return Result{}, err
	}
	_ = base.Close()

	sampler, err := nvml.New(nvml.Options{
		Now:      options.Now,
		Enricher: enricher,
	})
	if err != nil {
		_ = enricher.Close()
		return Result{}, err
	}
	return Result{Sampler: sampler, Notice: enricher.Notice()}, nil
}
```

- [ ] **Step 2: Add real GPU factory smoke test**

Create `internal/sampler/factory_gpu_test.go` with:

```go
//go:build gpu

package sampler

import (
	"context"
	"testing"
)

func TestFactorySmokeOnRealGPU(t *testing.T) {
	created, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer created.Sampler.Close()

	snapshot, err := created.Sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample error = %v", err)
	}
	if len(snapshot.Devices) == 0 {
		t.Fatalf("no GPUs found")
	}
}
```

- [ ] **Step 3: Wire CLI to sampler factory**

In `cmd/nvcoretop/main.go`, add import:

```go
	"nvcoretop/internal/sampler"
```

In the export branch, replace the fake sampler construction with:

```go
		created, err := sampler.New(sampler.Options{ForceDCGM: cfg.DCGM})
		if err != nil {
			return err
		}
		defer created.Sampler.Close()
		if created.Notice != "" {
			fmt.Fprintln(stderr, created.Notice)
		}
```

Then pass `created.Sampler` into `export.Run`.

In the TUI branch, replace the fake sampler construction with:

```go
		created, err := sampler.New(sampler.Options{ForceDCGM: cfg.DCGM})
		if err != nil {
			return err
		}
		defer created.Sampler.Close()
		return ui.Run(context.Background(), created.Sampler, cfg.Interval, ui.Options{
			Interval:      cfg.Interval.String(),
			NoColor:       cfg.NoColor,
			ForceDCGMView: cfg.DCGM,
		})
```

- [ ] **Step 4: Run default-build tests**

Run: `go test ./...`

Expected: PASS in the default build. No default test should call `sampler.New` because it requires NVML at runtime.

- [ ] **Step 5: Build binary**

Run: `CGO_ENABLED=1 go build ./cmd/nvcoretop`

Expected: PASS.

- [ ] **Step 6: Run real GPU smoke commands**

On a host with an NVIDIA driver, run:

```bash
./nvcoretop --json --count 1 --interval 100ms
./nvcoretop --csv --count 1 --interval 100ms
```

Expected: each command prints one record with at least one GPU.

- [ ] **Step 7: Run optional factory smoke test**

Run: `go test -tags gpu ./internal/sampler -run TestFactorySmokeOnRealGPU -v`

Expected on a GPU host with NVIDIA driver: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/sampler/factory.go internal/sampler/factory_gpu_test.go cmd/nvcoretop/main.go
git commit -m "feat: use real nvidia sampler"
```

## Self-Review Checklist

- Spec coverage: covers real NVML sampling, defensive unsupported fields, process permission limits, DCGM fallback and forced errors, power/clocks/throttle/fan/process/PCIe/NVLink/ECC fields, DCGM activity fields, and CLI replacement of fake sampler.
- Deferred beyond v1: remote aggregation, persistent logging, control operations, and non-NVIDIA GPUs remain absent.
- Unresolved-content scan: every code step provides concrete file content and every test step has an exact command with expected output.
- Type consistency: uses `gpu.Sampler`, `gpu.Enricher`, `gpu.Snapshot`, `gpu.DeviceSample`, `nvml.Sampler`, `dcgm.New`, and `sampler.New` consistently.
