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

func TestCoresViewUsesDCGMActivity(t *testing.T) {
	got := CoresView(gpu.DeviceSample{
		SMActivePct:      gpu.Some(82.5),
		TensorActivePct:  gpu.Some(12.0),
		MemPipeActivePct: gpu.Some(44.0),
		FP32ActivePct:    gpu.Some(65.0),
	}, true)
	for _, want := range []string{"SM", "Tensor", "MemPipe", "FP32"} {
		if !strings.Contains(got, want) {
			t.Fatalf("CoresView DCGM = %q, missing %q", got, want)
		}
	}
}
