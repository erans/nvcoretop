package export

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
)

func TestCSVEncoderWritesHeaderAndRows(t *testing.T) {
	fields, err := ResolveFields([]string{"util", "temp", "power"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	var buf bytes.Buffer
	encoder := NewCSVEncoder(&buf, fields, 2)
	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{
			{Index: 0, GPUUtil: gpu.Some(uint32(64)), TempC: gpu.Some(uint32(71)), PowerW: gpu.Some(285.5)},
			{Index: 1, GPUUtil: gpu.Some(uint32(12)), TempC: gpu.Some(uint32(44))},
		},
	})
	if err != nil {
		t.Fatalf("Write error = %v", err)
	}

	want := "ts,source,util_gpu0,temp_gpu0,power_gpu0,util_gpu1,temp_gpu1,power_gpu1\n2026-06-21T18:40:00Z,NVML,64,71,285.5,12,44,\n"
	if got := buf.String(); got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}

func TestCSVEncoderWritesStableWideLayoutAcrossWrites(t *testing.T) {
	fields, err := ResolveFields([]string{"util", "temp"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	var buf bytes.Buffer
	encoder := NewCSVEncoder(&buf, fields, 3)

	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{
			{Index: 0, GPUUtil: gpu.Some(uint32(10)), TempC: gpu.Some(uint32(20))},
			{Index: 2, GPUUtil: gpu.Some(uint32(30)), TempC: gpu.Some(uint32(40))},
		},
	})
	if err != nil {
		t.Fatalf("first Write error = %v", err)
	}
	if got := strings.Count(buf.String(), "\n"); got != 2 {
		t.Fatalf("newline count after first Write = %d, want header and first row flushed", got)
	}

	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 41, 0, 0, time.UTC),
		Source:    gpu.SourceNVMLDCGM,
		Devices: []gpu.DeviceSample{
			{Index: 2, GPUUtil: gpu.Some(uint32(55)), TempC: gpu.Some(uint32(66))},
		},
	})
	if err != nil {
		t.Fatalf("second Write error = %v", err)
	}

	raw := buf.String()
	headerLine := "ts,source,util_gpu0,temp_gpu0,util_gpu1,temp_gpu1,util_gpu2,temp_gpu2\n"
	if got := strings.Count(raw, headerLine); got != 1 {
		t.Fatalf("header count = %d, want 1", got)
	}

	reader := csv.NewReader(strings.NewReader(raw))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("csv parse error = %v", err)
	}

	want := [][]string{
		{"ts", "source", "util_gpu0", "temp_gpu0", "util_gpu1", "temp_gpu1", "util_gpu2", "temp_gpu2"},
		{"2026-06-21T18:40:00Z", "NVML", "10", "20", "", "", "30", "40"},
		{"2026-06-21T18:41:00Z", "NVML+DCGM", "", "", "", "", "55", "66"},
	}
	if len(records) != len(want) {
		t.Fatalf("record count = %d, want %d", len(records), len(want))
	}
	for i := range want {
		if len(records[i]) != len(want[i]) {
			t.Fatalf("record[%d] len = %d, want %d", i, len(records[i]), len(want[i]))
		}
		for j := range want[i] {
			if records[i][j] != want[i][j] {
				t.Fatalf("record[%d][%d] = %q, want %q", i, j, records[i][j], want[i][j])
			}
		}
	}
}

type failingWriter struct {
	err error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestCSVEncoderPropagatesWriterErrors(t *testing.T) {
	fields, err := ResolveFields([]string{"util"})
	if err != nil {
		t.Fatalf("ResolveFields error = %v", err)
	}

	expectedErr := errors.New("writer write failure")
	encoder := NewCSVEncoder(&failingWriter{err: expectedErr}, fields, 1)

	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 40, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{
			{Index: 0, GPUUtil: gpu.Some(uint32(64))},
		},
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Write error = %v, want %v", err, expectedErr)
	}

	err = encoder.Write(gpu.Snapshot{
		Timestamp: time.Date(2026, 6, 21, 18, 41, 0, 0, time.UTC),
		Source:    gpu.SourceNVML,
		Devices: []gpu.DeviceSample{
			{Index: 0, GPUUtil: gpu.Some(uint32(12))},
		},
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Write after flush error = %v, want %v", err, expectedErr)
	}
}
