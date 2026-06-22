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

func TestNextSortCycles(t *testing.T) {
	if got := SortIndex.Next(); got != SortUtil {
		t.Fatalf("SortIndex.Next() = %v, want SortUtil", got)
	}
	if got := SortPower.Next(); got != SortIndex {
		t.Fatalf("SortPower.Next() = %v, want SortIndex", got)
	}
}

func indexes(devices []gpu.DeviceSample) []int {
	out := make([]int, len(devices))
	for i, device := range devices {
		out[i] = device.Index
	}
	return out
}
