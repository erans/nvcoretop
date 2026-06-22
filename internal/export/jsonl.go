package export

import (
	"encoding/json"
	"io"

	"nvcoretop/internal/gpu"
)

type jsonlRecord struct {
	GPUs   []map[string]any `json:"gpus"`
	Source string           `json:"source"`
	TS     string           `json:"ts"`
}

func WriteJSONL(w io.Writer, snapshot gpu.Snapshot, fields []Field) error {
	record := jsonlRecord{
		GPUs:   make([]map[string]any, 0, len(snapshot.Devices)),
		Source: exportSource(snapshot.Source),
		TS:     snapshot.Timestamp.UTC().Format(timeRFC3339),
	}

	for _, device := range snapshot.Devices {
		row := make(map[string]any, len(fields))
		for _, field := range fields {
			row[field.Name] = field.Value(device).JSON
		}
		record.GPUs = append(record.GPUs, row)
	}

	encoder := json.NewEncoder(w)
	return encoder.Encode(record)
}

func exportSource(source gpu.Source) string {
	switch source {
	case gpu.SourceNVML:
		return "NVML"
	case gpu.SourceNVMLDCGM:
		return "NVML+DCGM"
	default:
		return "unknown"
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
