package ui

import (
	"context"
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
