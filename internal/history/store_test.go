package history

import (
	"testing"

	"nvcoretop/internal/gpu"
)

func TestStoreIngestsSupportedMetrics(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   2,
		GPUUtil: gpu.Some(uint32(60)),
		TempC:   gpu.Some(uint32(70)),
		PowerW:  gpu.Some(250.5),
	}}})
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   2,
		GPUUtil: gpu.Some(uint32(61)),
		TempC:   gpu.Some(uint32(71)),
		PowerW:  gpu.Some(251.5),
	}}})

	got, ok := store.Device(2)
	if !ok {
		t.Fatalf("Device(2) missing")
	}
	assertFloatSlice(t, got.Util.Values(), []float64{60, 61})
	assertFloatSlice(t, got.Temp.Values(), []float64{70, 71})
	assertFloatSlice(t, got.Power.Values(), []float64{250.5, 251.5})
}

func TestStoreSkipsMissingOptionalMetrics(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{Index: 0}}})

	got, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing")
	}
	assertFloatSlice(t, got.Util.Values(), nil)
	assertFloatSlice(t, got.Temp.Values(), nil)
	assertFloatSlice(t, got.Power.Values(), nil)
}

func TestStoreMissingDevice(t *testing.T) {
	store := NewStore(3)
	if _, ok := store.Device(99); ok {
		t.Fatalf("Device(99) present, want missing")
	}
}

func TestStorePreservesSampleContinuityAcrossMissingMetrics(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   0,
		GPUUtil: gpu.Some(uint32(10)),
		TempC:   gpu.Some(uint32(20)),
		PowerW:  gpu.Some(30.5),
	}}})
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{Index: 0}}})
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   0,
		GPUUtil: gpu.Some(uint32(11)),
		TempC:   gpu.Some(uint32(21)),
		PowerW:  gpu.Some(31.5),
	}}})

	got, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing")
	}
	assertFloatSlice(t, got.Util.Values(), []float64{10, 11})
	assertFloatSlice(t, got.Temp.Values(), []float64{20, 21})
	assertFloatSlice(t, got.Power.Values(), []float64{30.5, 31.5})
}

func TestStoreReturnsDeviceHistoryCopy(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   0,
		GPUUtil: gpu.Some(uint32(5)),
	}}})

	got, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing")
	}
	got.Util.Push(99)

	got2, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing after mutation of copy")
	}
	assertFloatSlice(t, got2.Util.Values(), []float64{5})
}

func TestStoreTracksDevicesIndependently(t *testing.T) {
	store := NewStore(3)
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:   0,
		GPUUtil: gpu.Some(uint32(10)),
		TempC:   gpu.Some(uint32(20)),
	}}})
	store.Add(gpu.Snapshot{Devices: []gpu.DeviceSample{{
		Index:  1,
		PowerW: gpu.Some(500.5),
	}}})

	got0, ok := store.Device(0)
	if !ok {
		t.Fatalf("Device(0) missing")
	}
	got1, ok := store.Device(1)
	if !ok {
		t.Fatalf("Device(1) missing")
	}

	assertFloatSlice(t, got0.Util.Values(), []float64{10})
	assertFloatSlice(t, got0.Temp.Values(), []float64{20})
	assertFloatSlice(t, got0.Power.Values(), nil)

	assertFloatSlice(t, got1.Util.Values(), nil)
	assertFloatSlice(t, got1.Temp.Values(), nil)
	assertFloatSlice(t, got1.Power.Values(), []float64{500.5})
}
