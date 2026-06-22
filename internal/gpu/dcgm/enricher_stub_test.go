//go:build !dcgm

package dcgm

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestNewStubFallsBackWhenNotForced(t *testing.T) {
	enricher, err := New(false, 1)
	if err != nil {
		t.Fatalf("New(false) error = %v", err)
	}
	if enricher.Active() {
		t.Fatalf("stub Active() = true, want false")
	}
	if notice := enricher.Notice(); !strings.Contains(notice, "DCGM unavailable") {
		t.Fatalf("Notice() = %q, want DCGM unavailable fallback notice", notice)
	}

	snapshot := gpu.Snapshot{
		Timestamp: time.Unix(10, 20),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{
			Index:   0,
			Name:    "GPU 0",
			GPUUtil: gpu.Some[uint32](42),
		}},
	}
	got, err := enricher.Enrich(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}
	if !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("Enrich() = %#v, want unchanged %#v", got, snapshot)
	}
	if err := enricher.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewStubErrorsWhenForced(t *testing.T) {
	_, err := New(true, 1)
	if err == nil || !strings.Contains(err.Error(), "not compiled") {
		t.Fatalf("New(true) error = %v, want not compiled error", err)
	}
}
