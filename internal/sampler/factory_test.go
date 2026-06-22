package sampler

import (
	"context"
	"errors"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
	"nvcoretop/internal/gpu/nvml"
)

func TestNewCreatesNVMLSamplerWithDCGMEnricher(t *testing.T) {
	base := &factoryTestSampler{deviceCount: 2}
	final := &factoryTestSampler{}
	enricher := &factoryTestEnricher{notice: "DCGM active"}
	fixedNow := time.Unix(1, 0).UTC()

	var nvmlOptions []nvml.Options
	useFactoryDependencies(t,
		func(options nvml.Options) (gpu.Sampler, error) {
			nvmlOptions = append(nvmlOptions, options)
			if len(nvmlOptions) == 1 {
				if options.Enricher != nil {
					t.Fatalf("first NVML Enricher = %#v, want nil", options.Enricher)
				}
				return base, nil
			}
			if options.Enricher != enricher {
				t.Fatalf("second NVML Enricher = %#v, want test enricher", options.Enricher)
			}
			return final, nil
		},
		func(force bool, deviceCount int) (gpu.Enricher, error) {
			if !force {
				t.Fatalf("force = false, want true")
			}
			if deviceCount != 2 {
				t.Fatalf("deviceCount = %d, want 2", deviceCount)
			}
			return enricher, nil
		},
	)

	created, err := New(Options{
		ForceDCGM: true,
		Now: func() time.Time {
			return fixedNow
		},
	})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	if created.Sampler != final {
		t.Fatalf("Sampler = %#v, want final sampler", created.Sampler)
	}
	if created.Notice != "DCGM active" {
		t.Fatalf("Notice = %q, want DCGM active", created.Notice)
	}
	if !base.closed {
		t.Fatalf("base sampler was not closed")
	}
	if enricher.closed {
		t.Fatalf("enricher was closed on success")
	}
	if len(nvmlOptions) != 2 {
		t.Fatalf("NVML calls = %d, want 2", len(nvmlOptions))
	}
	for i, options := range nvmlOptions {
		if options.Now == nil {
			t.Fatalf("NVML call %d Now = nil", i)
		}
		if got := options.Now(); !got.Equal(fixedNow) {
			t.Fatalf("NVML call %d Now() = %s, want %s", i, got, fixedNow)
		}
	}
}

func TestNewClosesBaseSamplerOnDCGMError(t *testing.T) {
	baseCloseErr := errors.New("base close failed")
	base := &factoryTestSampler{deviceCount: 1, closeErr: baseCloseErr}
	dcgmErr := errors.New("dcgm failed")

	useFactoryDependencies(t,
		func(nvml.Options) (gpu.Sampler, error) {
			return base, nil
		},
		func(bool, int) (gpu.Enricher, error) {
			return nil, dcgmErr
		},
	)

	_, err := New(Options{})
	if !errors.Is(err, dcgmErr) {
		t.Fatalf("New error = %v, want %v", err, dcgmErr)
	}
	if !errors.Is(err, baseCloseErr) {
		t.Fatalf("New error = %v, want %v", err, baseCloseErr)
	}
	if !base.closed {
		t.Fatalf("base sampler was not closed")
	}
}

func TestNewClosesEnricherOnBaseCloseFailure(t *testing.T) {
	baseCloseErr := errors.New("base close failed")
	enricherCloseErr := errors.New("enricher close failed")
	base := &factoryTestSampler{deviceCount: 1, closeErr: baseCloseErr}
	enricher := &factoryTestEnricher{closeErr: enricherCloseErr}
	call := 0

	useFactoryDependencies(t,
		func(options nvml.Options) (gpu.Sampler, error) {
			call++
			if call > 1 || options.Enricher != nil {
				t.Fatalf("second NVML creation occurred after base close failure")
			}
			return base, nil
		},
		func(bool, int) (gpu.Enricher, error) {
			return enricher, nil
		},
	)

	_, err := New(Options{})
	if !errors.Is(err, baseCloseErr) {
		t.Fatalf("New error = %v, want %v", err, baseCloseErr)
	}
	if !errors.Is(err, enricherCloseErr) {
		t.Fatalf("New error = %v, want %v", err, enricherCloseErr)
	}
	if !base.closed {
		t.Fatalf("base sampler was not closed")
	}
	if !enricher.closed {
		t.Fatalf("enricher was not closed")
	}
}

func TestNewClosesEnricherOnSecondNVMLFailure(t *testing.T) {
	base := &factoryTestSampler{deviceCount: 1}
	enricherCloseErr := errors.New("enricher close failed")
	enricher := &factoryTestEnricher{closeErr: enricherCloseErr}
	nvmlErr := errors.New("second nvml failed")
	call := 0

	useFactoryDependencies(t,
		func(nvml.Options) (gpu.Sampler, error) {
			call++
			if call == 1 {
				return base, nil
			}
			return nil, nvmlErr
		},
		func(bool, int) (gpu.Enricher, error) {
			return enricher, nil
		},
	)

	_, err := New(Options{})
	if !errors.Is(err, nvmlErr) {
		t.Fatalf("New error = %v, want %v", err, nvmlErr)
	}
	if !errors.Is(err, enricherCloseErr) {
		t.Fatalf("New error = %v, want %v", err, enricherCloseErr)
	}
	if !base.closed {
		t.Fatalf("base sampler was not closed")
	}
	if !enricher.closed {
		t.Fatalf("enricher was not closed")
	}
}

func useFactoryDependencies(
	t *testing.T,
	nvmlFactory func(nvml.Options) (gpu.Sampler, error),
	dcgmFactory func(bool, int) (gpu.Enricher, error),
) {
	t.Helper()

	originalNVML := newNVML
	originalDCGM := newDCGM
	newNVML = nvmlFactory
	newDCGM = dcgmFactory
	t.Cleanup(func() {
		newNVML = originalNVML
		newDCGM = originalDCGM
	})
}

type factoryTestSampler struct {
	deviceCount int
	closed      bool
	closeErr    error
}

func (s *factoryTestSampler) Sample(context.Context) (gpu.Snapshot, error) {
	return gpu.Snapshot{}, nil
}

func (s *factoryTestSampler) DeviceCount() int {
	return s.deviceCount
}

func (s *factoryTestSampler) Close() error {
	s.closed = true
	return s.closeErr
}

type factoryTestEnricher struct {
	notice   string
	closed   bool
	closeErr error
}

func (e *factoryTestEnricher) Enrich(_ context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	return snapshot, nil
}

func (e *factoryTestEnricher) Active() bool {
	return true
}

func (e *factoryTestEnricher) Notice() string {
	return e.notice
}

func (e *factoryTestEnricher) Close() error {
	e.closed = true
	return e.closeErr
}
