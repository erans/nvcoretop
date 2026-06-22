package ui

import (
	"testing"

	"nvcoretop/internal/gpu"
)

func TestSortDevicesByUtilDescending(t *testing.T) {
	devices := []gpu.DeviceSample{
		{Index: 0, GPUUtil: gpu.Some(uint32(10))},
		{Index: 1, GPUUtil: gpu.Some(uint32(90))},
		{Index: 2},
	}

	got := SortDevices(devices, SortUtil)
	if got[0].Index != 1 || got[1].Index != 0 || got[2].Index != 2 {
		t.Fatalf("sort by util order = %#v", indexes(got))
	}
}

func TestSortDevicesByIndexAscending(t *testing.T) {
	devices := []gpu.DeviceSample{{Index: 2}, {Index: 0}, {Index: 1}}
	got := SortDevices(devices, SortIndex)
	if got[0].Index != 0 || got[1].Index != 1 || got[2].Index != 2 {
		t.Fatalf("sort by index order = %#v", indexes(got))
	}
}

func TestSortDevicesByUtilIgnoresStaleMissingValues(t *testing.T) {
	devices := []gpu.DeviceSample{
		{Index: 0, GPUUtil: gpu.Some(uint32(10))},
		{Index: 2, GPUUtil: gpu.Optional[uint32]{Value: 99}},
		{Index: 1, GPUUtil: gpu.Optional[uint32]{Value: 1}},
	}

	got := SortDevices(devices, SortUtil)
	if got[0].Index != 0 || got[1].Index != 1 || got[2].Index != 2 {
		t.Fatalf("sort by util with missing stale values order = %#v", indexes(got))
	}
}

func TestSortDevicesByPowerIgnoresStaleMissingValues(t *testing.T) {
	devices := []gpu.DeviceSample{
		{Index: 0, PowerW: gpu.Some(10.0)},
		{Index: 2, PowerW: gpu.Optional[float64]{Value: 99.0}},
		{Index: 1, PowerW: gpu.Optional[float64]{Value: 1.0}},
	}

	got := SortDevices(devices, SortPower)
	if got[0].Index != 0 || got[1].Index != 1 || got[2].Index != 2 {
		t.Fatalf("sort by power with missing stale values order = %#v", indexes(got))
	}
}

func TestNextSortCycles(t *testing.T) {
	modes := []SortMode{SortIndex, SortUtil, SortTemp, SortMem, SortPower, SortIndex}
	for i := 0; i < len(modes)-1; i++ {
		if got := modes[i].Next(); got != modes[i+1] {
			t.Fatalf("%v.Next() = %v, want %v", modes[i], got, modes[i+1])
		}
	}
}

func indexes(devices []gpu.DeviceSample) []int {
	out := make([]int, len(devices))
	for i, device := range devices {
		out[i] = device.Index
	}
	return out
}
