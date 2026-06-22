# nvcoretop Core Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the CI-safe foundation for nvcoretop: Go module setup, GPU snapshot types, fake sampling, and rolling history.

**Architecture:** The `internal/gpu` package owns the sampler contract and all data types consumed by UI and export code. The `internal/history` package stores rolling metric windows by GPU index without knowing about NVML, DCGM, Bubble Tea, or encoders.

**Tech Stack:** Go 1.26, standard library tests, package-internal unit tests.

---

## File Structure

- Create: `go.mod` - declares the local module path and Go toolchain floor.
- Create: `.gitignore` - excludes local binaries and coverage files.
- Create: `internal/gpu/types.go` - core `Snapshot`, `DeviceSample`, optional value, process, throttle, sampler, and source types.
- Create: `internal/gpu/types_test.go` - tests optional value helpers and throttle formatting.
- Create: `internal/gpu/fake.go` - deterministic scripted sampler for tests.
- Create: `internal/gpu/fake_test.go` - tests fake sampler sequencing, errors, device count, and closure behavior.
- Create: `internal/history/ring.go` - fixed-size float ring buffer for one metric.
- Create: `internal/history/ring_test.go` - tests partial fill, wrap-around, and invalid sizes.
- Create: `internal/history/store.go` - per-device metric history used by UI sparklines.
- Create: `internal/history/store_test.go` - tests ingestion and missing device lookups.

## Decisions Locked In

- Use `module nvcoretop` until a remote import path exists.
- Represent unsupported values as `gpu.Optional[T]` so renderers and encoders must handle missing data explicitly.
- Keep memory values in bytes and power values in watts. NVML adapter code will convert milliwatts to watts in the hardware plan.
- Include `--fields` support in v1 through the export plan because selected field names can reuse the encoder field registry.

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `.gitignore`

- [ ] **Step 1: Create the module file**

Create `go.mod` with:

```go
module nvcoretop

go 1.24
```

- [ ] **Step 2: Create local ignore rules**

Create `.gitignore` with:

```gitignore
/nvcoretop
/dist/
/coverage.out
/*.test
```

- [ ] **Step 3: Verify empty module test run**

Run: `go test ./...`

Expected: `go: warning: "./..." matched no packages`

- [ ] **Step 4: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: initialize go module"
```

### Task 2: Define GPU Snapshot Types

**Files:**
- Create: `internal/gpu/types_test.go`
- Create: `internal/gpu/types.go`

- [ ] **Step 1: Write the failing type tests**

Create `internal/gpu/types_test.go` with:

```go
package gpu

import "testing"

func TestOptionalValues(t *testing.T) {
	missing := Optional[uint64]{}
	if missing.OK {
		t.Fatalf("zero optional should be missing")
	}

	present := Some(uint64(42))
	if !present.OK || present.Value != 42 {
		t.Fatalf("Some() = %#v, want present value 42", present)
	}
}

func TestThrottleReasonsActiveAndNames(t *testing.T) {
	reasons := ThrottleReasons{
		Power:   true,
		Thermal: true,
	}

	if !reasons.Active() {
		t.Fatalf("Active() = false, want true")
	}

	got := reasons.Names()
	want := []string{"power", "thermal"}
	if len(got) != len(want) {
		t.Fatalf("Names() length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSourceString(t *testing.T) {
	tests := map[Source]string{
		SourceNVML:     "NVML",
		SourceNVMLDCGM: "NVML+DCGM",
	}
	for source, want := range tests {
		if got := source.String(); got != want {
			t.Fatalf("%v.String() = %q, want %q", int(source), got, want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/gpu -run 'TestOptionalValues|TestThrottleReasonsActiveAndNames|TestSourceString' -v`

Expected: FAIL with undefined identifiers such as `Optional`, `Some`, `ThrottleReasons`, and `Source`.

- [ ] **Step 3: Implement the GPU model**

Create `internal/gpu/types.go` with:

```go
package gpu

import (
	"context"
	"time"
)

type Optional[T any] struct {
	Value T
	OK    bool
}

func Some[T any](value T) Optional[T] {
	return Optional[T]{Value: value, OK: true}
}

type Source int

const (
	SourceNVML Source = iota
	SourceNVMLDCGM
)

func (s Source) String() string {
	switch s {
	case SourceNVMLDCGM:
		return "NVML+DCGM"
	default:
		return "NVML"
	}
}

type Snapshot struct {
	Timestamp time.Time
	Source    Source
	Devices   []DeviceSample
}

type DeviceSample struct {
	Index int
	Name  string
	UUID  string

	MemUsed  uint64
	MemTotal uint64
	GPUUtil  Optional[uint32]
	MemUtil  Optional[uint32]
	TempC    Optional[uint32]

	PowerW      Optional[float64]
	PowerLimitW Optional[float64]

	SMClockMHz      Optional[uint32]
	MemClockMHz     Optional[uint32]
	ThrottleReasons ThrottleReasons
	FanPct          Optional[uint32]

	Processes []Process

	PCIeTxKBps    Optional[uint64]
	PCIeRxKBps    Optional[uint64]
	NVLinkTxKBps  Optional[uint64]
	NVLinkRxKBps  Optional[uint64]
	ECCSingleBit  Optional[uint64]
	ECCDoubleBit  Optional[uint64]
	SMActivePct   Optional[float64]
	TensorActivePct Optional[float64]
	MemPipeActivePct Optional[float64]
	FP32ActivePct Optional[float64]

	ProcessLimited bool
}

type Process struct {
	PID     uint32
	Name    string
	MemUsed uint64
}

type ThrottleReasons struct {
	GPUIdle          bool
	ApplicationsClocks bool
	SWPowerCap      bool
	HWSlowdown      bool
	SyncBoost       bool
	SWThermal       bool
	HWThermal       bool
	HWPowerBrake    bool
	Power           bool
	Thermal         bool
}

func (r ThrottleReasons) Active() bool {
	return r.GPUIdle ||
		r.ApplicationsClocks ||
		r.SWPowerCap ||
		r.HWSlowdown ||
		r.SyncBoost ||
		r.SWThermal ||
		r.HWThermal ||
		r.HWPowerBrake ||
		r.Power ||
		r.Thermal
}

func (r ThrottleReasons) Names() []string {
	names := make([]string, 0, 10)
	if r.GPUIdle {
		names = append(names, "idle")
	}
	if r.ApplicationsClocks {
		names = append(names, "app-clocks")
	}
	if r.SWPowerCap {
		names = append(names, "sw-power")
	}
	if r.HWSlowdown {
		names = append(names, "hw-slowdown")
	}
	if r.SyncBoost {
		names = append(names, "sync-boost")
	}
	if r.SWThermal {
		names = append(names, "sw-thermal")
	}
	if r.HWThermal {
		names = append(names, "hw-thermal")
	}
	if r.HWPowerBrake {
		names = append(names, "hw-power-brake")
	}
	if r.Power {
		names = append(names, "power")
	}
	if r.Thermal {
		names = append(names, "thermal")
	}
	return names
}

type Sampler interface {
	Sample(context.Context) (Snapshot, error)
	DeviceCount() int
	Close() error
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/gpu -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gpu/types.go internal/gpu/types_test.go
git commit -m "feat: add gpu snapshot model"
```

### Task 3: Add Scripted Fake Sampler

**Files:**
- Create: `internal/gpu/fake_test.go`
- Create: `internal/gpu/fake.go`

- [ ] **Step 1: Write fake sampler tests**

Create `internal/gpu/fake_test.go` with:

```go
package gpu

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFakeSamplerReturnsScriptedSnapshots(t *testing.T) {
	first := Snapshot{Timestamp: time.Unix(1, 0), Devices: []DeviceSample{{Index: 0, Name: "A"}}}
	second := Snapshot{Timestamp: time.Unix(2, 0), Devices: []DeviceSample{{Index: 0, Name: "B"}}}
	sampler := NewFakeSampler([]FakeStep{{Snapshot: first}, {Snapshot: second}})

	got, err := sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "A" {
		t.Fatalf("first sample = %#v, %v", got, err)
	}

	got, err = sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "B" {
		t.Fatalf("second sample = %#v, %v", got, err)
	}

	got, err = sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "B" {
		t.Fatalf("repeated last sample = %#v, %v", got, err)
	}
}

func TestFakeSamplerReturnsScriptedError(t *testing.T) {
	want := errors.New("sample failed")
	sampler := NewFakeSampler([]FakeStep{{Err: want}})

	_, got := sampler.Sample(context.Background())
	if !errors.Is(got, want) {
		t.Fatalf("Sample() error = %v, want %v", got, want)
	}
}

func TestFakeSamplerDeviceCountAndClose(t *testing.T) {
	sampler := NewFakeSampler([]FakeStep{{
		Snapshot: Snapshot{Devices: []DeviceSample{{Index: 0}, {Index: 1}}},
	}})

	if got := sampler.DeviceCount(); got != 2 {
		t.Fatalf("DeviceCount() = %d, want 2", got)
	}

	if err := sampler.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := sampler.Sample(context.Background())
	if !errors.Is(err, ErrSamplerClosed) {
		t.Fatalf("Sample() after close = %v, want ErrSamplerClosed", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/gpu -run TestFakeSampler -v`

Expected: FAIL with undefined `NewFakeSampler`, `FakeStep`, and `ErrSamplerClosed`.

- [ ] **Step 3: Implement fake sampler**

Create `internal/gpu/fake.go` with:

```go
package gpu

import (
	"context"
	"errors"
	"sync"
)

var ErrSamplerClosed = errors.New("sampler closed")

type FakeStep struct {
	Snapshot Snapshot
	Err      error
}

type FakeSampler struct {
	mu     sync.Mutex
	steps  []FakeStep
	next   int
	closed bool
}

func NewFakeSampler(steps []FakeStep) *FakeSampler {
	copied := make([]FakeStep, len(steps))
	copy(copied, steps)
	return &FakeSampler{steps: copied}
}

func (s *FakeSampler) Sample(context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return Snapshot{}, ErrSamplerClosed
	}
	if len(s.steps) == 0 {
		return Snapshot{}, nil
	}

	index := s.next
	if index >= len(s.steps) {
		index = len(s.steps) - 1
	} else {
		s.next++
	}
	step := s.steps[index]
	if step.Err != nil {
		return Snapshot{}, step.Err
	}
	return step.Snapshot, nil
}

func (s *FakeSampler) DeviceCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.steps) == 0 {
		return 0
	}
	return len(s.steps[0].Snapshot.Devices)
}

func (s *FakeSampler) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/gpu -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gpu/fake.go internal/gpu/fake_test.go
git commit -m "test: add scripted gpu sampler"
```

### Task 4: Add Fixed-Size Ring Buffer

**Files:**
- Create: `internal/history/ring_test.go`
- Create: `internal/history/ring.go`

- [ ] **Step 1: Write ring tests**

Create `internal/history/ring_test.go` with:

```go
package history

import "testing"

func TestRingPartialFill(t *testing.T) {
	r := NewRing(3)
	r.Push(10)
	r.Push(20)

	got := r.Values()
	want := []float64{10, 20}
	assertFloatSlice(t, got, want)
}

func TestRingWrapAround(t *testing.T) {
	r := NewRing(3)
	r.Push(10)
	r.Push(20)
	r.Push(30)
	r.Push(40)

	got := r.Values()
	want := []float64{20, 30, 40}
	assertFloatSlice(t, got, want)
}

func TestRingRejectsInvalidSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("NewRing(0) did not panic")
		}
	}()
	_ = NewRing(0)
}

func assertFloatSlice(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/history -run TestRing -v`

Expected: FAIL with undefined `NewRing`.

- [ ] **Step 3: Implement ring buffer**

Create `internal/history/ring.go` with:

```go
package history

type Ring struct {
	values []float64
	next   int
	full   bool
}

func NewRing(size int) *Ring {
	if size <= 0 {
		panic("history ring size must be positive")
	}
	return &Ring{values: make([]float64, size)}
}

func (r *Ring) Push(value float64) {
	r.values[r.next] = value
	r.next = (r.next + 1) % len(r.values)
	if r.next == 0 {
		r.full = true
	}
}

func (r *Ring) Values() []float64 {
	if !r.full {
		out := make([]float64, r.next)
		copy(out, r.values[:r.next])
		return out
	}

	out := make([]float64, len(r.values))
	copy(out, r.values[r.next:])
	copy(out[len(r.values)-r.next:], r.values[:r.next])
	return out
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/history -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/ring.go internal/history/ring_test.go
git commit -m "feat: add metric ring buffer"
```

### Task 5: Add Per-Device History Store

**Files:**
- Create: `internal/history/store_test.go`
- Create: `internal/history/store.go`

- [ ] **Step 1: Write store tests**

Create `internal/history/store_test.go` with:

```go
package history

import (
	"testing"

	"nvcoretop/internal/gpu"
)

func TestStoreIngestsSupportedMetrics(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:  2,
		GPUUtil: gpu.Some(uint32(60)),
		TempC:   gpu.Some(uint32(70)),
		PowerW:  gpu.Some(250.5),
	}}})
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:  2,
		GPUUtil: gpu.Some(uint32(61)),
		TempC:   gpu.Some(uint32(71)),
		PowerW:  gpu.Some(251.5),
	}}})

	got, ok := store.Device(2)
	if !ok {
		t.Fatalf("Device(2) missing")
	}
	assertFloatSlice(t, got.Util.Values(), []float64{60, 61})
	assertFloatSlice(t, got.Temp.Values(), []float64{70, 71})
	assertFloatSlice(t, got.Power.Values(), []float64{250.5, 251.5})
}

func TestStoreSkipsMissingOptionalMetrics(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{Index: 0}}})

	got, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing")
	}
	assertFloatSlice(t, got.Util.Values(), nil)
	assertFloatSlice(t, got.Temp.Values(), nil)
	assertFloatSlice(t, got.Power.Values(), nil)
}

func TestStoreMissingDevice(t *testing.T) {
	store := NewStore(3)
	if _, ok := store.Device(99); ok {
		t.Fatalf("Device(99) present, want missing")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/history -run TestStore -v`

Expected: FAIL with undefined `NewStore`.

- [ ] **Step 3: Implement store**

Create `internal/history/store.go` with:

```go
package history

import "nvcoretop/internal/gpu"

type DeviceHistory struct {
	Util  *Ring
	Temp  *Ring
	Power *Ring
}

type Store struct {
	window  int
	devices map[int]*DeviceHistory
}

func NewStore(window int) *Store {
	if window <= 0 {
		panic("history window must be positive")
	}
	return &Store{
		window:  window,
		devices: make(map[int]*DeviceHistory),
	}
}

func (s *Store) Add(snapshot gpu.Snapshot) {
	for _, device := range snapshot.Devices {
		history := s.ensure(device.Index)
		if device.GPUUtil.OK {
			history.Util.Push(float64(device.GPUUtil.Value))
		}
		if device.TempC.OK {
			history.Temp.Push(float64(device.TempC.Value))
		}
		if device.PowerW.OK {
			history.Power.Push(device.PowerW.Value)
		}
	}
}

func (s *Store) Device(index int) (DeviceHistory, bool) {
	history, ok := s.devices[index]
	if !ok {
		return DeviceHistory{}, false
	}
	return *history, true
}

func (s *Store) ensure(index int) *DeviceHistory {
	if history, ok := s.devices[index]; ok {
		return history
	}
	history := &DeviceHistory{
		Util:  NewRing(s.window),
		Temp:  NewRing(s.window),
		Power: NewRing(s.window),
	}
	s.devices[index] = history
	return history
}
```

- [ ] **Step 4: Run package tests**

Run: `go test ./internal/history -v`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/history/store.go internal/history/store_test.go
git commit -m "feat: track per-device metric history"
```

## Self-Review Checklist

- Spec coverage: covers `gpu` core types, `Sampler`, `FakeSampler`, optional unsupported values, and `history` ring buffers.
- Deferred to later plans: NVML/DCGM hardware access, export encoders, Bubble Tea UI, CLI mode dispatch.
- Unresolved-content scan: every code step provides concrete file content and every test step has an exact command with expected output.
- Type consistency: `Snapshot`, `DeviceSample`, `Optional`, `Process`, `ThrottleReasons`, `Sampler`, and `history.Store` names are used consistently by later plans.
