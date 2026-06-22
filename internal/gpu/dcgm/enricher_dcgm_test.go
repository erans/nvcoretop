//go:build dcgm && gpu

package dcgm

import (
	"context"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestRealDCGMEnricherSmoke(t *testing.T) {
	enricher, err := New(false, 1)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	defer enricher.Close()

	if !enricher.Active() {
		t.Skip("DCGM inactive on this host")
	}

	_, err = enricher.Enrich(context.Background(), gpu.Snapshot{
		Source:  gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{Index: 0}},
	})
	if err != nil {
		t.Fatalf("Enrich error = %v", err)
	}
}
