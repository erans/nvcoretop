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

func cloneProcesses(processes []Process) []Process {
	if len(processes) == 0 {
		return nil
	}

	copied := make([]Process, len(processes))
	copy(copied, processes)
	return copied
}

func cloneDeviceSamples(devices []DeviceSample) []DeviceSample {
	if len(devices) == 0 {
		return nil
	}

	copied := make([]DeviceSample, len(devices))
	for i, device := range devices {
		copied[i] = device
		copied[i].Processes = cloneProcesses(device.Processes)
	}
	return copied
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	copied := snapshot
	copied.Devices = cloneDeviceSamples(snapshot.Devices)
	return copied
}

type FakeSampler struct {
	mu     sync.Mutex
	steps  []FakeStep
	next   int
	closed bool
}

func NewFakeSampler(steps []FakeStep) *FakeSampler {
	copied := make([]FakeStep, len(steps))
	for i, step := range steps {
		copied[i] = step
		copied[i].Snapshot = cloneSnapshot(step.Snapshot)
	}
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
	return cloneSnapshot(step.Snapshot), nil
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
