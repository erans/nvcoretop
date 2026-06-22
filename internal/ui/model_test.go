package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"nvcoretop/internal/gpu"
)

func TestModelSelectionKeys(t *testing.T) {
	model := NewModel(Options{})
	model.snapshot = snapshotWithDevices(3)

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyDown})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if model.selected != 2 {
		t.Fatalf("selected = %d, want 2", model.selected)
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyUp})
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}
}

func TestModelToggleKeys(t *testing.T) {
	model := NewModel(Options{})

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyEnter})
	if !model.detail {
		t.Fatalf("detail = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !model.paused {
		t.Fatalf("paused = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !model.help {
		t.Fatalf("help = false, want true")
	}
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if !model.dcgmView {
		t.Fatalf("dcgmView = false, want true")
	}
}

func TestModelSnapshotMessageUpdatesHistory(t *testing.T) {
	model := NewModel(Options{HistoryWindow: 4})
	msg := SnapshotMsg(snapshotWithDevices(1))

	model, _ = updateModel(model, msg)
	if len(model.snapshot.Devices) != 1 {
		t.Fatalf("snapshot devices = %d, want 1", len(model.snapshot.Devices))
	}
	history, ok := model.history.Device(0)
	if !ok || len(history.Util.Values()) != 1 {
		t.Fatalf("history missing util value")
	}
}

func snapshotWithDevices(count int) gpu.Snapshot {
	devices := make([]gpu.DeviceSample, count)
	for i := range devices {
		devices[i] = gpu.DeviceSample{
			Index:    i,
			Name:     "RTX",
			MemUsed:  uint64(i+1) * 1024,
			MemTotal: 8192,
			GPUUtil:  gpu.Some(uint32(10 + i)),
			TempC:    gpu.Some(uint32(40 + i)),
			PowerW:   gpu.Some(float64(100 + i)),
		}
	}
	return gpu.Snapshot{Timestamp: time.Unix(1, 0).UTC(), Source: gpu.SourceNVML, Devices: devices}
}
