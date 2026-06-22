package export

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestWriteJSONL(t *testing.T) {
	fields, err := ResolveFields([]string{"i", "util", "temp", "power"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	snapshot := gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{
			Index:   0,
			GPUUtil: gpu.Some(uint32(64)),
			TempC:   gpu.Some(uint32(71)),
		}},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, snapshot, fields); err != nil {
		t.Fatalf("WriteJSONL error = %v", err)
	}

	want := "{\"gpus\":[{\"i\":0,\"power\":null,\"temp\":71,\"util\":64}],\"source\":\"NVML\",\"ts\":\"2026-06-21T18:40:00Z\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("JSONL = %q, want %q", got, want)
	}
}

func TestWriteJSONLSourceMapping(t *testing.T) {
	fields, err := ResolveFields([]string{"i"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}
	snapshot := gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Devices:   []gpu.DeviceSample{{Index: 0}},
	}

	for _, tt := range []struct {
		source gpu.Source
		want   string
	}{
		{gpu.SourceNVML, "NVML"},
		{gpu.SourceNVMLDCGM, "NVML+DCGM"},
		{gpu.Source(99), "unknown"},
	} {
		snapshot.Source = tt.source

		var buf bytes.Buffer
		if err := WriteJSONL(&buf, snapshot, fields); err != nil {
			t.Fatalf("WriteJSONL error = %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal error = %v", err)
		}
		if got["source"] != tt.want {
			t.Fatalf("source = %v, want %v", got["source"], tt.want)
		}
	}
}

func TestWriteJSONLNormalizesTimestampToUTC(t *testing.T) {
	fields, err := ResolveFields([]string{"i"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	snapshot := gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 11, 40, 0, 0, time.FixedZone("PST", -7*60*60)),
		Source:    gpu.SourceNVML,
		Devices:   []gpu.DeviceSample{{Index: 0}},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, snapshot, fields); err != nil {
		t.Fatalf("WriteJSONL error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	ts, ok := got["ts"].(string)
	if !ok {
		t.Fatalf("ts type = %T, want string", got["ts"])
	}
	if ts != "2026-06-21T18:40:00Z" {
		t.Fatalf("ts = %q, want %q", ts, "2026-06-21T18:40:00Z")
	}
}

func TestWriteJSONLMissingOptionalFieldsAsNull(t *testing.T) {
	fields, err := ResolveFields([]string{"i", "util", "pcie_tx"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	snapshot := gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVMLDCGM,
		Devices: []gpu.DeviceSample{{
			Index: 0,
		}},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, snapshot, fields); err != nil {
		t.Fatalf("WriteJSONL error = %v", err)
	}

	want := "{\"gpus\":[{\"i\":0,\"pcie_tx\":null,\"util\":null}],\"source\":\"NVML+DCGM\",\"ts\":\"2026-06-21T18:40:00Z\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("JSONL = %q, want %q", got, want)
	}
}
