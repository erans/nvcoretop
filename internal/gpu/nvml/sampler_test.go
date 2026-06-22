package nvml

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock"
	"nvcoretop/internal/gpu"
)

func TestNewInitFailureDoesNotShutdown(t *testing.T) {
	nvml := installFakeNVML(t, &fakeNVML{initRet: nvidia.ERROR_LIBRARY_NOT_FOUND})

	_, err := New(Options{})
	if err == nil || !strings.Contains(err.Error(), "NVML init failed") {
		t.Fatalf("New error = %v, want init failure", err)
	}
	if nvml.shutdownCalls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", nvml.shutdownCalls)
	}
}

func TestNewDeviceCountFailureShutsDown(t *testing.T) {
	nvml := installFakeNVML(t, &fakeNVML{
		countRet:    nvidia.ERROR_UNKNOWN,
		shutdownRet: nvidia.ERROR_NO_PERMISSION,
	})

	_, err := New(Options{})
	if err == nil || !strings.Contains(err.Error(), "NVML device count failed") {
		t.Fatalf("New error = %v, want device count failure", err)
	}
	if !strings.Contains(err.Error(), "NVML shutdown failed") {
		t.Fatalf("New error = %v, want joined shutdown failure", err)
	}
	if nvml.shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", nvml.shutdownCalls)
	}
}

func TestNewDeviceHandleFailureShutsDown(t *testing.T) {
	nvml := installFakeNVML(t, &fakeNVML{
		count:       2,
		handles:     []nvidia.Device{minimalMockDevice()},
		handleRets:  []nvidia.Return{nvidia.SUCCESS, nvidia.ERROR_NO_PERMISSION},
		shutdownRet: nvidia.ERROR_UNKNOWN,
	})

	_, err := New(Options{})
	if err == nil || !strings.Contains(err.Error(), "NVML device handle failed") {
		t.Fatalf("New error = %v, want device handle failure", err)
	}
	if !strings.Contains(err.Error(), "NVML shutdown failed") {
		t.Fatalf("New error = %v, want joined shutdown failure", err)
	}
	if nvml.shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", nvml.shutdownCalls)
	}
	if len(nvml.handleIndexes) != 2 || nvml.handleIndexes[0] != 0 || nvml.handleIndexes[1] != 1 {
		t.Fatalf("handle indexes = %v, want [0 1]", nvml.handleIndexes)
	}
}

func TestDeviceCountFromDiscoveredHandles(t *testing.T) {
	nvml := installFakeNVML(t, &fakeNVML{
		count:   2,
		handles: []nvidia.Device{minimalMockDevice(), minimalMockDevice()},
	})

	sampler, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer sampler.Close()

	if got := sampler.DeviceCount(); got != 2 {
		t.Fatalf("DeviceCount() = %d, want 2", got)
	}
	if len(nvml.handleIndexes) != 2 {
		t.Fatalf("handle indexes = %v, want two handles", nvml.handleIndexes)
	}
}

func TestSampleAfterCloseReturnsSamplerClosed(t *testing.T) {
	installFakeNVML(t, &fakeNVML{})
	sampler, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if err := sampler.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	_, err = sampler.Sample(context.Background())
	if !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("Sample error = %v, want ErrSamplerClosed", err)
	}
}

func TestSampleCanceledContextAfterCloseReturnsSamplerClosed(t *testing.T) {
	installFakeNVML(t, &fakeNVML{})
	sampler, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if err := sampler.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = sampler.Sample(ctx)
	if !errors.Is(err, gpu.ErrSamplerClosed) {
		t.Fatalf("Sample error = %v, want ErrSamplerClosed", err)
	}
}

func TestCloseJoinsErrorsAndReturnsSameErrorAgain(t *testing.T) {
	enricherErr := errors.New("enricher close failed")
	nvml := installFakeNVML(t, &fakeNVML{shutdownRet: nvidia.ERROR_UNKNOWN})
	sampler := &Sampler{
		now:      time.Now,
		enricher: &fakeEnricher{closeErr: enricherErr},
	}

	first := sampler.Close()
	if first == nil {
		t.Fatalf("Close error = nil, want joined error")
	}
	if !errors.Is(first, enricherErr) {
		t.Fatalf("Close error = %v, want joined enricher error", first)
	}
	if !strings.Contains(first.Error(), "NVML shutdown failed") {
		t.Fatalf("Close error = %v, want shutdown failure", first)
	}
	if nvml.shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", nvml.shutdownCalls)
	}

	second := sampler.Close()
	if second != first {
		t.Fatalf("second Close error = %v, want same error %v", second, first)
	}
	if nvml.shutdownCalls != 1 {
		t.Fatalf("shutdown calls after second Close = %d, want 1", nvml.shutdownCalls)
	}
}

func TestSampleReturnsPartialSnapshotOnEnricherError(t *testing.T) {
	enrichErr := errors.New("enrich failed")
	at := time.Date(2026, 6, 22, 8, 9, 10, 0, time.FixedZone("test", -7*60*60))
	sampler := &Sampler{
		devices:  []nvidia.Device{minimalMockDevice()},
		now:      func() time.Time { return at },
		enricher: &fakeEnricher{enrichErr: enrichErr},
	}

	snapshot, err := sampler.Sample(context.Background())
	if !errors.Is(err, enrichErr) {
		t.Fatalf("Sample error = %v, want enrich error", err)
	}
	if !snapshot.Timestamp.Equal(at.UTC()) {
		t.Fatalf("Timestamp = %v, want %v", snapshot.Timestamp, at.UTC())
	}
	if snapshot.Source != gpu.SourceNVML {
		t.Fatalf("Source = %v, want SourceNVML", snapshot.Source)
	}
	if len(snapshot.Devices) != 1 || snapshot.Devices[0].Name == "" {
		t.Fatalf("partial snapshot devices = %#v, want sampled device", snapshot.Devices)
	}
}

func TestSampleReturnsContextErrorBeforeSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sampler := &Sampler{
		devices: []nvidia.Device{&mock.Device{}},
		now:     time.Now,
	}

	_, err := sampler.Sample(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Sample error = %v, want context.Canceled", err)
	}
}

type fakeNVML struct {
	initRet     nvidia.Return
	count       int
	countRet    nvidia.Return
	handles     []nvidia.Device
	handleRet   nvidia.Return
	handleRets  []nvidia.Return
	shutdownRet nvidia.Return

	initCalls     int
	countCalls    int
	shutdownCalls int
	handleIndexes []int
}

func (f *fakeNVML) Init() nvidia.Return {
	f.initCalls++
	return f.initRet
}

func (f *fakeNVML) DeviceGetCount() (int, nvidia.Return) {
	f.countCalls++
	return f.count, f.countRet
}

func (f *fakeNVML) DeviceGetHandleByIndex(index int) (nvidia.Device, nvidia.Return) {
	f.handleIndexes = append(f.handleIndexes, index)
	if index < len(f.handleRets) && f.handleRets[index] != nvidia.SUCCESS {
		return nil, f.handleRets[index]
	}
	if f.handleRet != nvidia.SUCCESS {
		return nil, f.handleRet
	}
	if index < len(f.handles) {
		return f.handles[index], nvidia.SUCCESS
	}
	return minimalMockDevice(), nvidia.SUCCESS
}

func (f *fakeNVML) Shutdown() nvidia.Return {
	f.shutdownCalls++
	return f.shutdownRet
}

func installFakeNVML(t *testing.T, fake *fakeNVML) *fakeNVML {
	t.Helper()
	previous := nvmlClient
	nvmlClient = fake
	t.Cleanup(func() {
		nvmlClient = previous
	})
	return fake
}

type fakeEnricher struct {
	enrichErr error
	closeErr  error
}

func (f *fakeEnricher) Enrich(context.Context, gpu.Snapshot) (gpu.Snapshot, error) {
	return gpu.Snapshot{}, f.enrichErr
}

func (f *fakeEnricher) Active() bool {
	return true
}

func (f *fakeEnricher) Notice() string {
	return ""
}

func (f *fakeEnricher) Close() error {
	return f.closeErr
}
