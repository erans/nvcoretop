package ui

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"nvcoretop/internal/gpu"
)

func TestProgramRendersSnapshot(t *testing.T) {
	model := NewModel(Options{NoColor: true})
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	tm.Send(SnapshotMsg(gpu.Snapshot{
		Timestamp: time.Unix(1, 0).UTC(),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{
			Index:    0,
			Name:     "RTX 3090",
			MemUsed:  8 * 1024 * 1024 * 1024,
			MemTotal: 24 * 1024 * 1024 * 1024,
			GPUUtil:  gpu.Some(uint32(64)),
			TempC:    gpu.Some(uint32(71)),
			PowerW:   gpu.Some(285.0),
		}},
	}))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestSampleCmdSkipsWhenPaused(t *testing.T) {
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: snapshotWithDevices(1)}})
	cmd := sampleCmd(context.Background(), sampler, true)
	if msg := cmd(); msg != nil {
		t.Fatalf("paused sample cmd = %#v, want nil", msg)
	}
}

func TestRunRejectsNonPositiveInterval(t *testing.T) {
	sampler := gpu.NewFakeSampler(nil)
	if err := Run(context.Background(), sampler, 0, Options{}); !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("Run error = %v, want ErrInvalidInterval", err)
	}
}

func TestRunnerModelSamplesAndRendersSnapshot(t *testing.T) {
	snapshot := snapshotWithDevices(1)
	snapshot.Devices[0].Name = "RTX 3090"
	sampler := gpu.NewFakeSampler([]gpu.FakeStep{{Snapshot: snapshot}})
	tm := teatest.NewTestModel(t, runnerModel{
		Model:    NewModel(Options{NoColor: true}),
		ctx:      context.Background(),
		sampler:  sampler,
		interval: time.Millisecond,
	}, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("RTX 3090"))
	}, teatest.WithDuration(time.Second), teatest.WithCheckInterval(time.Millisecond))
}

func TestRunnerModelDoesNotOverlapSamples(t *testing.T) {
	release := make(chan struct{})
	sampler := newBlockingSampler(snapshotWithDevices(1), release)
	tm := teatest.NewTestModel(t, runnerModel{
		Model:    NewModel(Options{NoColor: true}),
		ctx:      context.Background(),
		sampler:  sampler,
		interval: 5 * time.Millisecond,
	}, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() {
		close(release)
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	})

	sampler.waitForSamples(t, 1)
	time.Sleep(30 * time.Millisecond)
	if max := sampler.maxConcurrent(); max != 1 {
		t.Fatalf("max concurrent samples = %d, want 1", max)
	}
}

type blockingSampler struct {
	mu        sync.Mutex
	snapshot  gpu.Snapshot
	release   <-chan struct{}
	started   chan struct{}
	active    int
	maxActive int
}

func newBlockingSampler(snapshot gpu.Snapshot, release <-chan struct{}) *blockingSampler {
	return &blockingSampler{
		snapshot: snapshot,
		release:  release,
		started:  make(chan struct{}, 16),
	}
}

func (s *blockingSampler) Sample(ctx context.Context) (gpu.Snapshot, error) {
	s.mu.Lock()
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()

	s.started <- struct{}{}
	select {
	case <-ctx.Done():
		return gpu.Snapshot{}, ctx.Err()
	case <-s.release:
	}

	return s.snapshot, nil
}

func (s *blockingSampler) DeviceCount() int {
	return len(s.snapshot.Devices)
}

func (s *blockingSampler) Close() error {
	return nil
}

func (s *blockingSampler) waitForSamples(t *testing.T, want int) {
	t.Helper()
	for samples := 0; samples < want; samples++ {
		select {
		case <-s.started:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for sample %d", samples+1)
		}
	}
}

func (s *blockingSampler) maxConcurrent() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxActive
}
