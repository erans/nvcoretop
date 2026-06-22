//go:build gpu

package sampler

import (
	"context"
	"testing"
)

func TestFactorySmokeOnRealGPU(t *testing.T) {
	created, err := New(Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer created.Sampler.Close()

	snapshot, err := created.Sampler.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample error = %v", err)
	}
	if len(snapshot.Devices) == 0 {
		t.Fatalf("no GPUs found")
	}
}
