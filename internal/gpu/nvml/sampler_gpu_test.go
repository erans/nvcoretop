//go:build gpu

package nvml

import (
	"context"
	"testing"
)

func TestSamplerSmokeOnRealGPU(t *testing.T) {
	sampler, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer func() {
		if err := sampler.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
	}()

	snapshot, err := sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample error = %v", err)
	}
	if len(snapshot.Devices) == 0 {
		t.Fatalf("no GPUs found")
	}
	if snapshot.Devices[0].Name == "" {
		t.Fatalf("first GPU name empty: %#v", snapshot.Devices[0])
	}
}
