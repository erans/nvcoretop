package export

import (
	"bytes"
	"context"
	"errors"
	"strings"
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
			Format:    FormatJSONL,
			Interval:  time.Second,
			Count:     2,
			Fields:    []string{"i", "util"},
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	err := Run(ctx, sampler, &buf, Options{
		Format:    FormatJSONL,
		Count:     1,
		NewTicker: tickerFactory(newManualTicker()),
	})
	if err != context.Canceled {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
}

func TestRunStopsOnDuration(t *testing.T) {
	sampler := &blockingSampler{
		started: make(chan struct{}, 1),
	}
	ticker := newManualTicker()

	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), sampler, &bytes.Buffer{}, Options{
			Format:    FormatJSONL,
			Duration:  20 * time.Millisecond,
			NewTicker: tickerFactory(ticker),
		})
	}()

	ticker.Tick()
	sampler.waitForStart(t)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run error = %v, want nil", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Run did not stop after duration")
	}
}

func TestRunReturnsCSVFlushError(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC(), Devices: []gpu.DeviceSample{{Index: 0, GPUUtil: gpu.Some(uint32(10))}}}}})
	ticker := newManualTicker()

	flushErr := errors.New("flush failed")
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), sampler, &flushWriter{err: flushErr}, Options{
			Format:    FormatCSV,
			Count:     1,
			Fields:    []string{"i", "util"},
			NewTicker: tickerFactory(ticker),
		})
	}()

	ticker.Tick()

	err := <-done
	if !errors.Is(err, flushErr) {
		t.Fatalf("Run error = %v, want %v", err, flushErr)
	}
}

func TestRunRejectsUnsupportedFormat(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC()}}})
	ticker := newManualTicker()
	var buf bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), sampler, &buf, Options{
			Format:    Format(99),
			Count:     1,
			NewTicker: tickerFactory(ticker),
		})
	}()

	ticker.Tick()

	err := <-done
	if err == nil || !strings.Contains(err.Error(), "unsupported export format") {
		t.Fatalf("Run error = %v, want unsupported export format", err)
	}
}

type blockingSampler struct {
	started chan struct{}
}

func (s *blockingSampler) Sample(ctx context.Context) (gpu.Snapshot, error) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return gpu.Snapshot{}, ctx.Err()
}

func (s *blockingSampler) DeviceCount() int {
	return 1
}

func (s *blockingSampler) Close() error {
	return nil
}

func (s *blockingSampler) waitForStart(t *testing.T) {
	t.Helper()
	select {
	case <-s.started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Sample did not start")
	}
}

type flushWriter struct {
	err error
}

func (w *flushWriter) Write(p []byte) (int, error) {
	return len(p), w.err
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
