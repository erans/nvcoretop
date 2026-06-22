package nvml

import (
	"context"
	"errors"
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
	mu         sync.Mutex
	devices    []nvidia.Device
	now        func() time.Time
	enricher   gpu.Enricher
	lastNVLink map[int]nvlinkTotals
	closed     bool
	closeErr   error
}

func New(options Options) (*Sampler, error) {
	if ret := nvmlClient.Init(); !ok(ret) {
		return nil, errString("NVML init failed", ret)
	}

	count, ret := nvmlClient.DeviceGetCount()
	if !ok(ret) {
		return nil, errors.Join(errString("NVML device count failed", ret), shutdownError())
	}

	devices := make([]nvidia.Device, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvmlClient.DeviceGetHandleByIndex(i)
		if !ok(ret) {
			return nil, errors.Join(errString("NVML device handle failed", ret), shutdownError())
		}
		devices = append(devices, device)
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &Sampler{
		devices:    devices,
		now:        now,
		enricher:   options.Enricher,
		lastNVLink: make(map[int]nvlinkTotals),
	}, nil
}

func (s *Sampler) Sample(ctx context.Context) (gpu.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return gpu.Snapshot{}, gpu.ErrSamplerClosed
	}
	if err := ctx.Err(); err != nil {
		return gpu.Snapshot{}, err
	}

	snapshot := gpu.Snapshot{
		Timestamp: s.now().UTC(),
		Source:    gpu.SourceNVML,
		Devices:   make([]gpu.DeviceSample, 0, len(s.devices)),
	}
	for index, device := range s.devices {
		deviceSample := sampleDevice(index, device, processNameFromProc, nil)
		if totals, found := readNVLinkTotals(device, snapshot.Timestamp); found {
			if previous, ok := s.lastNVLink[index]; ok {
				applyNVLinkDelta(&deviceSample, previous, totals)
			}
			if s.lastNVLink == nil {
				s.lastNVLink = make(map[int]nvlinkTotals)
			}
			s.lastNVLink[index] = totals
		}
		snapshot.Devices = append(snapshot.Devices, deviceSample)
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
		return s.closeErr
	}
	s.closed = true

	var errs []error
	if s.enricher != nil {
		if err := s.enricher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if ret := nvmlClient.Shutdown(); !ok(ret) {
		errs = append(errs, errString("NVML shutdown failed", ret))
	}
	s.closeErr = errors.Join(errs...)
	return s.closeErr
}

func shutdownError() error {
	if ret := nvmlClient.Shutdown(); !ok(ret) {
		return errString("NVML shutdown failed", ret)
	}
	return nil
}

type nvmlAPI interface {
	Init() nvidia.Return
	DeviceGetCount() (int, nvidia.Return)
	DeviceGetHandleByIndex(int) (nvidia.Device, nvidia.Return)
	Shutdown() nvidia.Return
}

type realNVML struct{}

func (realNVML) Init() nvidia.Return {
	return nvidia.Init()
}

func (realNVML) DeviceGetCount() (int, nvidia.Return) {
	return nvidia.DeviceGetCount()
}

func (realNVML) DeviceGetHandleByIndex(index int) (nvidia.Device, nvidia.Return) {
	return nvidia.DeviceGetHandleByIndex(index)
}

func (realNVML) Shutdown() nvidia.Return {
	return nvidia.Shutdown()
}

var nvmlClient nvmlAPI = realNVML{}
