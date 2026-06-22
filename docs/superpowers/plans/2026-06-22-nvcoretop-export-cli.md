# nvcoretop Export and CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add scriptable JSONL and CSV export mode plus CLI flag parsing and validation.

**Architecture:** Export encoders consume the `gpu.Snapshot` model from the core plan and never call GPU APIs directly. CLI parsing lives in `internal/app`, producing a `Config` that `cmd/nvcoretop` can dispatch to export mode now and TUI mode after the UI plan.

**Tech Stack:** Go standard library (`encoding/json`, `encoding/csv`, `flag`, `time`, `context`), `internal/gpu` fake sampler for deterministic tests.

---

## File Structure

- Modify: `go.mod` - already created by the core plan.
- Create: `internal/export/fields.go` - field registry, default field list, and `--fields` validation.
- Create: `internal/export/fields_test.go` - tests default fields, subset ordering, and unknown field errors.
- Create: `internal/export/jsonl.go` - JSONL encoder, including JSON `null` for unsupported optional values.
- Create: `internal/export/jsonl_test.go` - golden-style JSONL test with deterministic timestamp.
- Create: `internal/export/csv.go` - wide CSV encoder with fixed card count and suffixed per-GPU columns.
- Create: `internal/export/csv_test.go` - header and row tests for two GPUs.
- Create: `internal/export/runner.go` - count/duration-aware sampler loop shared by CLI export modes.
- Create: `internal/export/runner_test.go` - deterministic runner tests with fake sampler and manual ticker.
- Create: `internal/app/config.go` - CLI config, flag parsing, and validation.
- Create: `internal/app/config_test.go` - flag validation tests.
- Create: `cmd/nvcoretop/main.go` - binary entrypoint for export mode and temporary TUI-not-linked message.

## Prerequisite

Complete `docs/superpowers/plans/2026-06-22-nvcoretop-core.md` first. This plan imports `nvcoretop/internal/gpu`.

## Decisions Locked In

- `--json` and `--csv` are mutually exclusive.
- `--fields` ships in v1. Field names are validated at startup and preserve caller order.
- JSON unsupported values encode as `null`; CSV unsupported values encode as an empty cell.
- CSV process data uses fixed columns `proc_count` and `proc_mem` to keep the header stable.
- `cmd/nvcoretop` returns a clear non-zero message for interactive mode until the TUI plan wires Bubble Tea.

### Task 1: Add Export Field Registry

**Files:**
- Create: `internal/export/fields_test.go`
- Create: `internal/export/fields.go`

- [ ] **Step 1: Write field registry tests**

Create `internal/export/fields_test.go` with:

```go
package export

import (
	"errors"
	"testing"
)

func TestResolveFieldsDefault(t *testing.T) {
	fields, err := ResolveFields(nil)
	if err != nil {
		t.Fatalf("ResolveFields(nil) error = %v", err)
	}
	if len(fields) == 0 {
		t.Fatalf("default fields empty")
	}
	if fields[0].Name != "i" || fields[1].Name != "name" || fields[2].Name != "uuid" {
		t.Fatalf("first default fields = %q, %q, %q", fields[0].Name, fields[1].Name, fields[2].Name)
	}
}

func TestResolveFieldsSubsetPreservesOrder(t *testing.T) {
	fields, err := ResolveFields([]string{"temp", "util", "power"})
	if err != nil {
		t.Fatalf("ResolveFields subset error = %v", err)
	}
	got := []string{fields[0].Name, fields[1].Name, fields[2].Name}
	want := []string{"temp", "util", "power"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveFieldsUnknown(t *testing.T) {
	_, err := ResolveFields([]string{"util", "bogus"})
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("ResolveFields unknown error = %v, want ErrUnknownField", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/export -run TestResolveFields -v`

Expected: FAIL with undefined `ResolveFields` and `ErrUnknownField`.

- [ ] **Step 3: Implement field registry**

Create `internal/export/fields.go` with:

```go
package export

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"nvcoretop/internal/gpu"
)

var ErrUnknownField = errors.New("unknown export field")

type Field struct {
	Name  string
	Value func(gpu.DeviceSample) FieldValue
}

type FieldValue struct {
	JSON any
	CSV  string
}

var fieldRegistry = map[string]Field{
	"i": {Name: "i", Value: func(d gpu.DeviceSample) FieldValue {
		return intValue(d.Index)
	}},
	"name": {Name: "name", Value: func(d gpu.DeviceSample) FieldValue {
		return stringValue(d.Name)
	}},
	"uuid": {Name: "uuid", Value: func(d gpu.DeviceSample) FieldValue {
		return stringValue(d.UUID)
	}},
	"util": {Name: "util", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.GPUUtil)
	}},
	"mem_util": {Name: "mem_util", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.MemUtil)
	}},
	"mem_used": {Name: "mem_used", Value: func(d gpu.DeviceSample) FieldValue {
		return uint64Value(d.MemUsed)
	}},
	"mem_total": {Name: "mem_total", Value: func(d gpu.DeviceSample) FieldValue {
		return uint64Value(d.MemTotal)
	}},
	"temp": {Name: "temp", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.TempC)
	}},
	"power": {Name: "power", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.PowerW)
	}},
	"power_limit": {Name: "power_limit", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.PowerLimitW)
	}},
	"sm_clock": {Name: "sm_clock", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.SMClockMHz)
	}},
	"mem_clock": {Name: "mem_clock", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.MemClockMHz)
	}},
	"fan": {Name: "fan", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.FanPct)
	}},
	"proc_count": {Name: "proc_count", Value: func(d gpu.DeviceSample) FieldValue {
		return intValue(len(d.Processes))
	}},
	"proc_mem": {Name: "proc_mem", Value: func(d gpu.DeviceSample) FieldValue {
		var total uint64
		for _, proc := range d.Processes {
			total += proc.MemUsed
		}
		return uint64Value(total)
	}},
	"pcie_tx": {Name: "pcie_tx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.PCIeTxKBps)
	}},
	"pcie_rx": {Name: "pcie_rx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.PCIeRxKBps)
	}},
	"nvlink_tx": {Name: "nvlink_tx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.NVLinkTxKBps)
	}},
	"nvlink_rx": {Name: "nvlink_rx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.NVLinkRxKBps)
	}},
	"ecc_sbe": {Name: "ecc_sbe", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.ECCSingleBit)
	}},
	"ecc_dbe": {Name: "ecc_dbe", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.ECCDoubleBit)
	}},
	"sm_active": {Name: "sm_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.SMActivePct)
	}},
	"tensor_active": {Name: "tensor_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.TensorActivePct)
	}},
	"mem_pipe_active": {Name: "mem_pipe_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.MemPipeActivePct)
	}},
	"fp32_active": {Name: "fp32_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.FP32ActivePct)
	}},
}

var defaultFieldNames = []string{
	"i", "name", "uuid",
	"util", "mem_util", "mem_used", "mem_total", "temp",
	"power", "power_limit", "sm_clock", "mem_clock", "fan",
	"proc_count", "proc_mem",
	"pcie_tx", "pcie_rx", "nvlink_tx", "nvlink_rx",
	"ecc_sbe", "ecc_dbe",
	"sm_active", "tensor_active", "mem_pipe_active", "fp32_active",
}

func ResolveFields(names []string) ([]Field, error) {
	if len(names) == 0 {
		names = defaultFieldNames
	}

	fields := make([]Field, 0, len(names))
	for _, name := range names {
		normalized := strings.TrimSpace(name)
		field, ok := fieldRegistry[normalized]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownField, normalized)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func intValue(value int) FieldValue {
	return FieldValue{JSON: value, CSV: strconv.Itoa(value)}
}

func uint64Value(value uint64) FieldValue {
	return FieldValue{JSON: value, CSV: strconv.FormatUint(value, 10)}
}

func stringValue(value string) FieldValue {
	return FieldValue{JSON: value, CSV: value}
}

func optionalUint32(value gpu.Optional[uint32]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatUint(uint64(value.Value), 10)}
}

func optionalUint64(value gpu.Optional[uint64]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatUint(value.Value, 10)}
}

func optionalFloat(value gpu.Optional[float64]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatFloat(value.Value, 'f', -1, 64)}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/export -run TestResolveFields -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/fields.go internal/export/fields_test.go
git commit -m "feat: add export field registry"
```

### Task 2: Add JSONL Encoder

**Files:**
- Create: `internal/export/jsonl_test.go`
- Create: `internal/export/jsonl.go`

- [ ] **Step 1: Write JSONL encoder test**

Create `internal/export/jsonl_test.go` with:

```go
package export

import (
	"bytes"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestWriteJSONL(t *testing.T) {
	fields, err := ResolveFields([]string{"i", "util", "temp", "power"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	snapshot := gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{
			Index: 0,
			GPUUtil: gpu.Some(uint32(64)),
			TempC: gpu.Some(uint32(71)),
		}},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, snapshot, fields); err != nil {
		t.Fatalf("WriteJSONL error = %v", err)
	}

	want := "{\"gpus\":[{\"i\":0,\"power\":null,\"temp\":71,\"util\":64}],\"source\":\"NVML\",\"ts\":\"2026-06-21T18:40:00Z\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("JSONL = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/export -run TestWriteJSONL -v`

Expected: FAIL with undefined `WriteJSONL`.

- [ ] **Step 3: Implement JSONL encoder**

Create `internal/export/jsonl.go` with:

```go
package export

import (
	"encoding/json"
	"io"

	"nvcoretop/internal/gpu"
)

type jsonlRecord struct {
	GPUs   []map[string]any `json:"gpus"`
	Source string           `json:"source"`
	TS     string           `json:"ts"`
}

func WriteJSONL(w io.Writer, snapshot gpu.Snapshot, fields []Field) error {
	record := jsonlRecord{
		GPUs:   make([]map[string]any, 0, len(snapshot.Devices)),
		Source: snapshot.Source.String(),
		TS:     snapshot.Timestamp.UTC().Format(timeRFC3339),
	}

	for _, device := range snapshot.Devices {
		row := make(map[string]any, len(fields))
		for _, field := range fields {
			row[field.Name] = field.Value(device).JSON
		}
		record.GPUs = append(record.GPUs, row)
	}

	encoder := json.NewEncoder(w)
	return encoder.Encode(record)
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
```

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/export -run TestWriteJSONL -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/jsonl.go internal/export/jsonl_test.go
git commit -m "feat: add jsonl exporter"
```

### Task 3: Add Wide CSV Encoder

**Files:**
- Create: `internal/export/csv_test.go`
- Create: `internal/export/csv.go`

- [ ] **Step 1: Write CSV encoder test**

Create `internal/export/csv_test.go` with:

```go
package export

import (
	"bytes"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestCSVEncoderWritesHeaderAndRows(t *testing.T) {
	fields, err := ResolveFields([]string{"util", "temp", "power"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	var buf bytes.Buffer
	encoder := NewCSVEncoder(&buf, fields, 2)
	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{
			{Index: 0, GPUUtil: gpu.Some(uint32(64)), TempC: gpu.Some(uint32(71)), PowerW: gpu.Some(285.5)},
			{Index: 1, GPUUtil: gpu.Some(uint32(12)), TempC: gpu.Some(uint32(44))},
		},
	})
	if err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if err := encoder.Flush(); err != nil {
		t.Fatalf("Flush error = %v", err)
	}

	want := "ts,source,util_gpu0,temp_gpu0,power_gpu0,util_gpu1,temp_gpu1,power_gpu1\n2026-06-21T18:40:00Z,NVML,64,71,285.5,12,44,\n"
	if got := buf.String(); got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/export -run TestCSVEncoderWritesHeaderAndRows -v`

Expected: FAIL with undefined `NewCSVEncoder`.

- [ ] **Step 3: Implement CSV encoder**

Create `internal/export/csv.go` with:

```go
package export

import (
	"encoding/csv"
	"fmt"
	"io"

	"nvcoretop/internal/gpu"
)

type CSVEncoder struct {
	writer      *csv.Writer
	fields      []Field
	deviceCount int
	wroteHeader bool
}

func NewCSVEncoder(w io.Writer, fields []Field, deviceCount int) *CSVEncoder {
	return &CSVEncoder{
		writer:      csv.NewWriter(w),
		fields:      fields,
		deviceCount: deviceCount,
	}
}

func (e *CSVEncoder) Write(snapshot gpu.Snapshot) error {
	if !e.wroteHeader {
		if err := e.writer.Write(e.header()); err != nil {
			return err
		}
		e.wroteHeader = true
	}

	byIndex := make(map[int]gpu.DeviceSample, len(snapshot.Devices))
	for _, device := range snapshot.Devices {
		byIndex[device.Index] = device
	}

	row := []string{snapshot.Timestamp.UTC().Format(timeRFC3339), snapshot.Source.String()}
	for i := 0; i < e.deviceCount; i++ {
		device, ok := byIndex[i]
		for _, field := range e.fields {
			if !ok {
				row = append(row, "")
				continue
			}
			row = append(row, field.Value(device).CSV)
		}
	}
	return e.writer.Write(row)
}

func (e *CSVEncoder) Flush() error {
	e.writer.Flush()
	return e.writer.Error()
}

func (e *CSVEncoder) header() []string {
	header := []string{"ts", "source"}
	for i := 0; i < e.deviceCount; i++ {
		for _, field := range e.fields {
			header = append(header, fmt.Sprintf("%s_gpu%d", field.Name, i))
		}
	}
	return header
}
```

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/export -run TestCSVEncoderWritesHeaderAndRows -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/csv.go internal/export/csv_test.go
git commit -m "feat: add csv exporter"
```

### Task 4: Add Export Runner

**Files:**
- Create: `internal/export/runner_test.go`
- Create: `internal/export/runner.go`

- [ ] **Step 1: Write runner tests**

Create `internal/export/runner_test.go` with:

```go
package export

import (
	"bytes"
	"context"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestRunJSONLCount(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{
		{Snapshot: gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC(), Source: gpu.SourceNVML, Devices: []gpu.DeviceSample{{Index: 0, GPUUtil: gpu.Some(uint32(10))}}}},
		{Snapshot: gpu.Snapshot{Timestamp: time.Unix(2, 0).UTC(), Source: gpu.SourceNVML, Devices: []gpu.DeviceSample{{Index: 0, GPUUtil: gpu.Some(uint32(20))}}}},
	})
	ticker := newManualTicker()
	var buf bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), sampler, &buf, Options{
			Format:   FormatJSONL,
			Interval: time.Second,
			Count:    2,
			Fields:   []string{"i", "util"},
			NewTicker: tickerFactory(ticker),
		})
	}()

	ticker.Tick()
	ticker.Tick()

	if err := <-done; err != nil {
		t.Fatalf("Run error = %v", err)
	}

	got := buf.String()
	want := "{\"gpus\":[{\"i\":0,\"util\":10}],\"source\":\"NVML\",\"ts\":\"1970-01-01T00:00:01Z\"}\n{\"gpus\":[{\"i\":0,\"util\":20}],\"source\":\"NVML\",\"ts\":\"1970-01-01T00:00:02Z\"}\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunStopsOnContext(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC()}}})
	ticker := newManualTicker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	err := Run(ctx, sampler, &buf, Options{
		Format: FormatJSONL,
		Count:  1,
		NewTicker: tickerFactory(ticker),
	})
	if err != context.Canceled {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
}

type manualTicker struct {
	ch chan time.Time
}

func newManualTicker() *manualTicker {
	return &manualTicker{ch: make(chan time.Time, 4)}
}

func (t *manualTicker) C() <-chan time.Time {
	return t.ch
}

func (t *manualTicker) Stop() {}

func (t *manualTicker) Tick() {
	t.ch <- time.Now()
}

func tickerFactory(t *manualTicker) func(time.Duration) Ticker {
	return func(time.Duration) Ticker { return t }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/export -run TestRun -v`

Expected: FAIL with undefined `Run`, `Options`, `FormatJSONL`, and `Ticker`.

- [ ] **Step 3: Implement runner**

Create `internal/export/runner.go` with:

```go
package export

import (
	"context"
	"fmt"
	"io"
	"time"

	"nvcoretop/internal/gpu"
)

type Format int

const (
	FormatJSONL Format = iota
	FormatCSV
)

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	ticker *time.Ticker
}

func (t realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t realTicker) Stop() {
	t.ticker.Stop()
}

type Options struct {
	Format    Format
	Interval  time.Duration
	Duration  time.Duration
	Count     int
	Fields    []string
	NewTicker func(time.Duration) Ticker
}

func Run(ctx context.Context, sampler gpu.Sampler, w io.Writer, opts Options) error {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	if opts.NewTicker == nil {
		opts.NewTicker = func(interval time.Duration) Ticker {
			return realTicker{ticker: time.NewTicker(interval)}
		}
	}

	fields, err := ResolveFields(opts.Fields)
	if err != nil {
		return err
	}

	var deadline <-chan time.Time
	var cancel context.CancelFunc
	if opts.Duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Duration)
		defer cancel()
		deadline = ctx.Done()
	}
	_ = deadline

	ticker := opts.NewTicker(opts.Interval)
	defer ticker.Stop()

	var csvEncoder *CSVEncoder
	if opts.Format == FormatCSV {
		csvEncoder = NewCSVEncoder(w, fields, sampler.DeviceCount())
		defer csvEncoder.Flush()
	}

	written := 0
	for {
		if opts.Count > 0 && written >= opts.Count {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C():
			snapshot, err := sampler.Sample(ctx)
			if err != nil {
				return err
			}
			switch opts.Format {
			case FormatJSONL:
				err = WriteJSONL(w, snapshot, fields)
			case FormatCSV:
				err = csvEncoder.Write(snapshot)
			default:
				err = fmt.Errorf("unsupported export format: %d", opts.Format)
			}
			if err != nil {
				return err
			}
			written++
		}
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/export -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/runner.go internal/export/runner_test.go
git commit -m "feat: add export sampling loop"
```

### Task 5: Add CLI Config Parsing

**Files:**
- Create: `internal/app/config_test.go`
- Create: `internal/app/config.go`

- [ ] **Step 1: Write config tests**

Create `internal/app/config_test.go` with:

```go
package app

import (
	"errors"
	"testing"
	"time"
)

func TestParseArgsJSON(t *testing.T) {
	cfg, err := ParseArgs([]string{"--json", "--interval", "2s", "--count", "3", "--fields", "util,temp"})
	if err != nil {
		t.Fatalf("ParseArgs error = %v", err)
	}
	if cfg.Mode != ModeJSON || cfg.Interval != 2*time.Second || cfg.Count != 3 {
		t.Fatalf("config = %#v", cfg)
	}
	if len(cfg.Fields) != 2 || cfg.Fields[0] != "util" || cfg.Fields[1] != "temp" {
		t.Fatalf("fields = %#v", cfg.Fields)
	}
}

func TestParseArgsRejectsJSONAndCSV(t *testing.T) {
	_, err := ParseArgs([]string{"--json", "--csv"})
	if !errors.Is(err, ErrMutuallyExclusiveFormat) {
		t.Fatalf("ParseArgs error = %v, want ErrMutuallyExclusiveFormat", err)
	}
}

func TestParseArgsRejectsBadInterval(t *testing.T) {
	_, err := ParseArgs([]string{"--json", "--interval", "0s"})
	if !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("ParseArgs error = %v, want ErrInvalidInterval", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app -run TestParseArgs -v`

Expected: FAIL with undefined `ParseArgs`, `ModeJSON`, `ErrMutuallyExclusiveFormat`, and `ErrInvalidInterval`.

- [ ] **Step 3: Implement config parser**

Create `internal/app/config.go` with:

```go
package app

import (
	"errors"
	"flag"
	"strings"
	"time"
)

var (
	ErrMutuallyExclusiveFormat = errors.New("--json and --csv are mutually exclusive")
	ErrInvalidInterval        = errors.New("--interval must be positive")
	ErrInvalidCount           = errors.New("--count cannot be negative")
)

type Mode int

const (
	ModeTUI Mode = iota
	ModeJSON
	ModeCSV
)

type Config struct {
	Mode     Mode
	Interval time.Duration
	Output   string
	Duration time.Duration
	Count    int
	Fields   []string
	DCGM     bool
	NoColor  bool
	Version  bool
}

func ParseArgs(args []string) (Config, error) {
	cfg := Config{
		Mode:     ModeTUI,
		Interval: time.Second,
		Output:   "-",
	}

	flags := flag.NewFlagSet("nvcoretop", flag.ContinueOnError)
	jsonMode := flags.Bool("json", false, "stream JSONL")
	csvMode := flags.Bool("csv", false, "stream CSV")
	fields := flags.String("fields", "", "comma-separated export fields")
	flags.DurationVar(&cfg.Interval, "interval", time.Second, "sample interval")
	flags.StringVar(&cfg.Output, "output", "-", "export destination")
	flags.DurationVar(&cfg.Duration, "duration", 0, "export duration")
	flags.IntVar(&cfg.Count, "count", 0, "export sample count")
	flags.BoolVar(&cfg.DCGM, "dcgm", false, "force DCGM activity")
	flags.BoolVar(&cfg.NoColor, "no-color", false, "disable color")
	flags.BoolVar(&cfg.Version, "version", false, "print version")

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if *jsonMode && *csvMode {
		return Config{}, ErrMutuallyExclusiveFormat
	}
	if cfg.Interval <= 0 {
		return Config{}, ErrInvalidInterval
	}
	if cfg.Count < 0 {
		return Config{}, ErrInvalidCount
	}

	switch {
	case *jsonMode:
		cfg.Mode = ModeJSON
	case *csvMode:
		cfg.Mode = ModeCSV
	}

	if strings.TrimSpace(*fields) != "" {
		for _, field := range strings.Split(*fields, ",") {
			cfg.Fields = append(cfg.Fields, strings.TrimSpace(field))
		}
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/app -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "feat: parse cli configuration"
```

### Task 6: Add Binary Entrypoint

**Files:**
- Create: `cmd/nvcoretop/main.go`

- [ ] **Step 1: Create entrypoint**

Create `cmd/nvcoretop/main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"nvcoretop/internal/app"
	"nvcoretop/internal/export"
	"nvcoretop/internal/gpu"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cfg, err := app.ParseArgs(args)
	if err != nil {
		return err
	}
	if cfg.Version {
		fmt.Fprintf(stdout, "nvcoretop %s\n", version)
		return nil
	}

	switch cfg.Mode {
	case app.ModeJSON, app.ModeCSV:
		format := export.FormatJSONL
		if cfg.Mode == app.ModeCSV {
			format = export.FormatCSV
		}
		sampler := gpu.NewFakeSampler([]gpu.FakeStep{{
			Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
		}})
		defer sampler.Close()
		if cfg.Output != "-" {
			file, err := os.Create(cfg.Output)
			if err != nil {
				return err
			}
			defer file.Close()
			stdout = file
		}
		return export.Run(context.Background(), sampler, stdout, export.Options{
			Format:   format,
			Interval: cfg.Interval,
			Duration: cfg.Duration,
			Count:    cfg.Count,
			Fields:   cfg.Fields,
		})
	default:
		fmt.Fprintln(stderr, "interactive TUI mode will be enabled by the UI plan")
		return nil
	}
}
```

- [ ] **Step 2: Build binary**

Run: `go build ./cmd/nvcoretop`

Expected: PASS and creates `./nvcoretop`.

- [ ] **Step 3: Run export smoke command**

Run: `./nvcoretop --json --count 1 --interval 1ms`

Expected: one JSONL record with empty `gpus` and source `NVML`.

- [ ] **Step 4: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/nvcoretop/main.go
git commit -m "feat: add nvcoretop cli entrypoint"
```

## Self-Review Checklist

- Spec coverage: covers `--json`, `--csv`, `--interval`, `--output`, `--duration`, `--count`, `--fields`, `--dcgm`, `--no-color`, `--version`, JSONL one object per tick, CSV wide header, stdout/file output, mutually exclusive export modes, and graceful auto-stop.
- Deferred to later plans: replacing the temporary fake sampler with NVML/DCGM factory, interactive TUI mode, and terminal color auto-detection.
- Unresolved-content scan: every code step provides concrete file content and every test step has an exact command with expected output.
- Type consistency: uses `gpu.Snapshot`, `gpu.DeviceSample`, `gpu.Sampler`, and `export.Options` consistently.
