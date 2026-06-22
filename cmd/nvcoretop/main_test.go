package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
	"nvcoretop/internal/sampler"
	"nvcoretop/internal/ui"
)

func useSamplerFactory(t *testing.T, factory func(sampler.Options) (sampler.Result, error)) {
	t.Helper()

	original := createSampler
	createSampler = factory
	t.Cleanup(func() {
		createSampler = original
	})
}

func useRuntimeContext(t *testing.T, factory func() (context.Context, context.CancelFunc)) {
	t.Helper()

	original := newRuntimeContext
	newRuntimeContext = factory
	t.Cleanup(func() {
		newRuntimeContext = original
	})
}

func failOnSamplerFactory(t *testing.T) {
	t.Helper()

	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		t.Fatalf("createSampler was called")
		return sampler.Result{}, nil
	})
}

func TestRunVersion(t *testing.T) {
	failOnSamplerFactory(t)

	var stdout, stderr bytes.Buffer

	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if got, want := stdout.String(), "nvcoretop dev\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	fakeSampler := gpu.NewFakeSampler([]gpu.FakeStep{{
		Snapshot: gpu.Snapshot{
			Timestamp: time.Unix(1, 0).UTC(),
			Source:    gpu.SourceNVML,
			Devices: []gpu.DeviceSample{{
				Index:   0,
				Name:    "RTX test",
				UUID:    "GPU-test",
				GPUUtil: gpu.Some(uint32(42)),
			}},
		},
	}})
	factoryCalls := 0
	useSamplerFactory(t, func(options sampler.Options) (sampler.Result, error) {
		factoryCalls++
		if options.ForceDCGM {
			t.Fatalf("ForceDCGM = true, want false")
		}
		return sampler.Result{Sampler: fakeSampler, Notice: "test notice"}, nil
	})

	if err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if factoryCalls != 1 {
		t.Fatalf("factoryCalls = %d, want 1", factoryCalls)
	}
	if got, want := strings.TrimSpace(stderr.String()), "test notice"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}

	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'})
	if len(lines) != 1 {
		t.Fatalf("record count = %d, want 1; output = %q", len(lines), stdout.String())
	}

	var got map[string]any
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if got["source"] != "NVML" {
		t.Fatalf("source = %v, want NVML", got["source"])
	}
	gpus, ok := got["gpus"].([]any)
	if !ok {
		t.Fatalf("gpus type = %T, want []any", got["gpus"])
	}
	if len(gpus) != 1 {
		t.Fatalf("gpus = %#v, want one device", gpus)
	}
	if _, err := fakeSampler.Sample(context.Background()); !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("sampler.Sample after run error = %v, want ErrSamplerClosed", err)
	}
}

func TestRunExportUsesRuntimeContext(t *testing.T) {
	var stdout, stderr bytes.Buffer
	fakeSampler := gpu.NewFakeSampler([]gpu.FakeStep{{
		Snapshot: gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC(), Source: gpu.SourceNVML},
	}})
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{Sampler: fakeSampler}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopCalled := false
	useRuntimeContext(t, func() (context.Context, context.CancelFunc) {
		return ctx, func() { stopCalled = true }
	})

	err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run error = %v, want nil graceful cancellation", err)
	}
	if !stopCalled {
		t.Fatalf("runtime context stop was not called")
	}
	if _, err := fakeSampler.Sample(context.Background()); !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("sampler.Sample after run error = %v, want ErrSamplerClosed", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunExportRuntimeCancelPreservesSamplerCloseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	want := errors.New("close failed")
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{Sampler: closeErrorSampler{closeErr: want}}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	useRuntimeContext(t, func() (context.Context, context.CancelFunc) {
		return ctx, func() {}
	})

	err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr)
	if !errors.Is(err, want) {
		t.Fatalf("run error = %v, want sampler close error %v", err, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDefaultModeDispatchesTUI(t *testing.T) {
	originalRunTUI := runTUI
	defer func() {
		runTUI = originalRunTUI
	}()

	var stdout, stderr bytes.Buffer
	called := false
	fakeSampler := gpu.NewFakeSampler([]gpu.FakeStep{{
		Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
	}})
	factoryCalls := 0
	useSamplerFactory(t, func(options sampler.Options) (sampler.Result, error) {
		factoryCalls++
		if !options.ForceDCGM {
			t.Fatalf("ForceDCGM = false, want true")
		}
		return sampler.Result{Sampler: fakeSampler}, nil
	})
	runTUI = func(ctx context.Context, gpuSampler gpu.Sampler, interval time.Duration, options ui.Options) error {
		t.Helper()
		called = true

		if ctx == nil {
			t.Fatalf("ctx = nil, want context")
		}
		if interval != 250*time.Millisecond {
			t.Fatalf("interval = %s, want 250ms", interval)
		}
		if options.Interval != "250ms" {
			t.Fatalf("options.Interval = %q, want 250ms", options.Interval)
		}
		if !options.NoColor {
			t.Fatalf("options.NoColor = false, want true")
		}
		if !options.ForceDCGMView {
			t.Fatalf("options.ForceDCGMView = false, want true")
		}

		snapshot, err := gpuSampler.Sample(ctx)
		if err != nil {
			t.Fatalf("sampler.Sample error = %v", err)
		}
		if snapshot.Source != gpu.SourceNVML {
			t.Fatalf("snapshot.Source = %s, want NVML", snapshot.Source)
		}
		return nil
	}

	if err := run([]string{"--interval", "250ms", "--no-color", "--dcgm"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if !called {
		t.Fatalf("runTUI was not called")
	}
	if factoryCalls != 1 {
		t.Fatalf("factoryCalls = %d, want 1", factoryCalls)
	}
	if _, err := fakeSampler.Sample(context.Background()); !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("sampler.Sample after run error = %v, want ErrSamplerClosed", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

type runtimeContextKey struct{}

func TestRunDefaultModeUsesRuntimeContext(t *testing.T) {
	originalRunTUI := runTUI
	defer func() {
		runTUI = originalRunTUI
	}()

	var stdout, stderr bytes.Buffer
	fakeSampler := gpu.NewFakeSampler([]gpu.FakeStep{{
		Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
	}})
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{Sampler: fakeSampler}, nil
	})

	stopCalled := false
	useRuntimeContext(t, func() (context.Context, context.CancelFunc) {
		return context.WithValue(context.Background(), runtimeContextKey{}, "runtime"), func() { stopCalled = true }
	})

	runTUI = func(ctx context.Context, gpuSampler gpu.Sampler, interval time.Duration, options ui.Options) error {
		if got := ctx.Value(runtimeContextKey{}); got != "runtime" {
			t.Fatalf("ctx value = %v, want runtime", got)
		}
		return nil
	}

	if err := run([]string{"--interval", "250ms"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}
	if !stopCalled {
		t.Fatalf("runtime context stop was not called")
	}
	if _, err := fakeSampler.Sample(context.Background()); !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("sampler.Sample after run error = %v, want ErrSamplerClosed", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDefaultModeTreatsRuntimeContextCancelAsClean(t *testing.T) {
	originalRunTUI := runTUI
	defer func() {
		runTUI = originalRunTUI
	}()

	var stdout, stderr bytes.Buffer
	fakeSampler := gpu.NewFakeSampler([]gpu.FakeStep{{
		Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
	}})
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{Sampler: fakeSampler}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	useRuntimeContext(t, func() (context.Context, context.CancelFunc) {
		return ctx, func() {}
	})

	runTUI = func(ctx context.Context, gpuSampler gpu.Sampler, interval time.Duration, options ui.Options) error {
		return ctx.Err()
	}

	if err := run([]string{"--interval", "250ms"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v, want nil graceful cancellation", err)
	}
	if _, err := fakeSampler.Sample(context.Background()); !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("sampler.Sample after run error = %v, want ErrSamplerClosed", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunFactoryErrorPropagates(t *testing.T) {
	var stdout, stderr bytes.Buffer
	want := errors.New("factory failed")
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{}, want
	})

	err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr)
	if !errors.Is(err, want) {
		t.Fatalf("run error = %v, want %v", err, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunExportReturnsSamplerCloseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	want := errors.New("close failed")
	useSamplerFactory(t, func(sampler.Options) (sampler.Result, error) {
		return sampler.Result{
			Sampler: closeErrorSampler{closeErr: want},
		}, nil
	})

	err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr)
	if !errors.Is(err, want) {
		t.Fatalf("run error = %v, want %v", err, want)
	}
	if stdout.Len() == 0 {
		t.Fatalf("stdout = empty, want exported sample before close error")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	failOnSamplerFactory(t)

	var stdout, stderr bytes.Buffer

	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{"nvcoretop", "--json", "--csv", "--dcgm", "--no-color"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output = %q, want to contain %q", got, want)
		}
	}
}

func TestRunInvalidFlag(t *testing.T) {
	failOnSamplerFactory(t)

	var stdout, stderr bytes.Buffer

	err := run([]string{"--does-not-exist"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run error = nil, want error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

type closeErrorSampler struct {
	closeErr error
}

func (s closeErrorSampler) Sample(context.Context) (gpu.Snapshot, error) {
	return gpu.Snapshot{
		Timestamp: time.Unix(1, 0).UTC(),
		Source:    gpu.SourceNVML,
	}, nil
}

func (s closeErrorSampler) DeviceCount() int {
	return 0
}

func (s closeErrorSampler) Close() error {
	return s.closeErr
}
