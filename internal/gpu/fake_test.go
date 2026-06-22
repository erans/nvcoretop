package gpu

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFakeSamplerReturnsScriptedSnapshots(t *testing.T) {
	first := Snapshot{Timestamp: time.Unix(1, 0), Devices: []DeviceSample{{Index: 0, Name: "A"}}}
	second := Snapshot{Timestamp: time.Unix(2, 0), Devices: []DeviceSample{{Index: 0, Name: "B"}}}
	sampler := NewFakeSampler([]FakeStep{{Snapshot: first}, {Snapshot: second}})

	got, err := sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "A" {
		t.Fatalf("first sample = %#v, %v", got, err)
	}

	got, err = sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "B" {
		t.Fatalf("second sample = %#v, %v", got, err)
	}

	got, err = sampler.Sample(context.Background())
	if err != nil || got.Devices[0].Name != "B" {
		t.Fatalf("repeated last sample = %#v, %v", got, err)
	}
}

func TestFakeSamplerReturnsScriptedError(t *testing.T) {
	want := errors.New("sample failed")
	sampler := NewFakeSampler([]FakeStep{{Err: want}})

	_, got := sampler.Sample(context.Background())
	if !errors.Is(got, want) {
		t.Fatalf("Sample() error = %v, want %v", got, want)
	}
}

func TestFakeSamplerDeviceCountAndClose(t *testing.T) {
	sampler := NewFakeSampler([]FakeStep{{
		Snapshot: Snapshot{Devices: []DeviceSample{{Index: 0}, {Index: 1}}},
	}})

	if got := sampler.DeviceCount(); got != 2 {
		t.Fatalf("DeviceCount() = %d, want 2", got)
	}

	if err := sampler.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := sampler.Sample(context.Background())
	if !errors.Is(err, ErrSamplerClosed) {
		t.Fatalf("Sample() after close = %v, want ErrSamplerClosed", err)
	}
}

func TestFakeSamplerIsolatedFromInputMutation(t *testing.T) {
	script := []FakeStep{{
		Snapshot: Snapshot{
			Devices: []DeviceSample{{
				Index: 0,
				Name:  "A",
				Processes: []Process{
					{PID: 1, Name: "app"},
				},
			}},
		},
	}}
	sampler := NewFakeSampler(script)

	script[0].Snapshot.Devices[0].Name = "mutated"
	script[0].Snapshot.Devices[0].Processes[0].PID = 99
	script[0].Snapshot.Devices = append(script[0].Snapshot.Devices, DeviceSample{Index: 1, Name: "extra"})

	got, err := sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample() error = %v", err)
	}
	if got.Devices[0].Name != "A" {
		t.Fatalf("sample name = %q, want %q", got.Devices[0].Name, "A")
	}
	if got.Devices[0].Processes[0].PID != 1 {
		t.Fatalf("sample process pid = %d, want %d", got.Devices[0].Processes[0].PID, 1)
	}
	if len(got.Devices) != 1 {
		t.Fatalf("sample device count = %d, want %d", len(got.Devices), 1)
	}
}

func TestFakeSamplerSampleReturnedSnapshotIsolation(t *testing.T) {
	sampler := NewFakeSampler([]FakeStep{{
		Snapshot: Snapshot{
			Devices: []DeviceSample{{
				Index: 0,
				Name:  "A",
				Processes: []Process{
					{PID: 1, Name: "app"},
				},
			}},
		},
	}})

	got, err := sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("first Sample() error = %v", err)
	}

	got.Devices[0].Name = "mutated"
	got.Devices[0].Processes[0].PID = 99

	got, err = sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("second Sample() error = %v", err)
	}
	if got.Devices[0].Name != "A" {
		t.Fatalf("second sample name = %q, want %q", got.Devices[0].Name, "A")
	}
	if got.Devices[0].Processes[0].PID != 1 {
		t.Fatalf("second sample process pid = %d, want %d", got.Devices[0].Processes[0].PID, 1)
	}
}
