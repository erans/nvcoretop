package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"nvcoretop/internal/gpu"
)

func TestModelNewDefaults(t *testing.T) {
	model := NewModel(Options{})

	if model.options.HistoryWindow != defaultHistoryWindow {
		t.Fatalf("options.HistoryWindow = %d, want %d", model.options.HistoryWindow, defaultHistoryWindow)
	}
	if model.sort != SortIndex {
		t.Fatalf("sort = %v, want %v", model.sort, SortIndex)
	}
	if model.dcgmView {
		t.Fatalf("dcgmView = true, want false")
	}
	if model.history == nil {
		t.Fatalf("history = nil, want store")
	}

	for range defaultHistoryWindow + 1 {
		model.history.Add(snapshotWithDevices(1))
	}
	history, ok := model.history.Device(0)
	if !ok {
		t.Fatalf("history missing device")
	}
	if got := len(history.Util.Values()); got != defaultHistoryWindow {
		t.Fatalf("history window length = %d, want %d", got, defaultHistoryWindow)
	}
}

func TestModelNewOptions(t *testing.T) {
	model := NewModel(Options{HistoryWindow: 4, ForceDCGMView: true})

	if model.options.HistoryWindow != 4 {
		t.Fatalf("options.HistoryWindow = %d, want 4", model.options.HistoryWindow)
	}
	if !model.dcgmView {
		t.Fatalf("dcgmView = false, want true")
	}
}

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

func TestModelTensorWallKeys(t *testing.T) {
	model := NewModel(Options{})
	model.detail = true

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if model.view != viewTensorWall {
		t.Fatalf("view = %v, want tensor wall", model.view)
	}
	if !model.detail {
		t.Fatalf("detail = false, want preserved while entering tensor wall")
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if model.view != viewOverview {
		t.Fatalf("view = %v, want overview after toggling tensor wall off", model.view)
	}
	if !model.detail {
		t.Fatalf("detail = false, want t toggle to preserve previous detail state")
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if model.view != viewOverview {
		t.Fatalf("view = %v, want overview after o", model.view)
	}
	if model.detail {
		t.Fatalf("detail = true, want o to return to overview without detail")
	}
}

func TestModelSortKeyCyclesSortMode(t *testing.T) {
	model := NewModel(Options{})

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if model.sort != SortUtil {
		t.Fatalf("sort = %v, want %v", model.sort, SortUtil)
	}
}

func TestModelQuitKeysReturnCommand(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{
			name: "q",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
		},
		{
			name: "ctrl+c",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := updateModel(NewModel(Options{}), tt.msg)
			if cmd == nil {
				t.Fatalf("cmd = nil, want quit command")
			}
		})
	}
}

func TestModelClampSelectionForSnapshotSizes(t *testing.T) {
	model := NewModel(Options{})
	model.selected = 5

	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(0)))
	if model.selected != 0 {
		t.Fatalf("selected after empty snapshot = %d, want 0", model.selected)
	}

	model.selected = 2
	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(1)))
	if model.selected != 0 {
		t.Fatalf("selected after shrink = %d, want 0", model.selected)
	}
}

func TestModelWindowSizeMessageUpdatesDimensions(t *testing.T) {
	model := NewModel(Options{})

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 40})
	if model.width != 100 {
		t.Fatalf("width = %d, want 100", model.width)
	}
	if model.height != 40 {
		t.Fatalf("height = %d, want 40", model.height)
	}
}

func TestModelErrMsgStoresError(t *testing.T) {
	model := NewModel(Options{})
	err := errors.New("poll failed")

	model, _ = updateModel(model, ErrMsg{Err: err})
	if !errors.Is(model.err, err) {
		t.Fatalf("err = %v, want %v", model.err, err)
	}
}

func TestModelSnapshotMessageClearsError(t *testing.T) {
	model := NewModel(Options{})
	model.err = errors.New("poll failed")

	model, _ = updateModel(model, SnapshotMsg(snapshotWithDevices(1)))
	if model.err != nil {
		t.Fatalf("err = %v, want nil", model.err)
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
