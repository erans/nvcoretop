package ui

import (
	"strings"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestCoresGridUsesGPUUtilFallback(t *testing.T) {
	got := CoresView(gpu.DeviceSample{GPUUtil: gpu.Some(uint32(50))}, false)
	if !strings.Contains(got, "cores") || !strings.Contains(got, "50%") {
		t.Fatalf("CoresView fallback = %q", got)
	}
}

func TestCoresViewUsesDCGMTileBlocks(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct:      gpu.Some(83.0),
		TensorActivePct:  gpu.Some(42.0),
		MemPipeActivePct: gpu.Some(58.0),
		FP32ActivePct:    gpu.Some(16.0),
	}, true)

	for _, want := range []string{
		"Core Activity",
		"SM 83%",
		"Tensor Pipe 42%",
		"DRAM 58%",
		"FP32 16%",
		"██████████░░  █████░░░░░░░",
		"███████░░░░░  ██░░░░░░░░░░",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView DCGM tiles = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "MemPipe") {
		t.Fatalf("CoresView DCGM tiles should label field 1005 as DRAM, got %q", got)
	}
}

func TestCoresViewShowsMissingDCGMFieldsAsEmptyTiles(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct: gpu.Some(50.0),
	}, true)

	for _, want := range []string{
		"SM 50%",
		"Tensor Pipe n/a",
		"DRAM n/a",
		"FP32 n/a",
		"██████░░░░░░  ░░░░░░░░░░░░",
		"░░░░░░░░░░░░  ░░░░░░░░░░░░",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView missing DCGM tiles = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "MemPipe") {
		t.Fatalf("CoresView missing DCGM tiles should label field 1005 as DRAM, got %q", got)
	}
}
