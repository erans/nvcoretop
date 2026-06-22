package export

import (
	"encoding/csv"
	"fmt"
	"io"

	"nvcoretop/internal/gpu"
)

type CSVEncoder struct {
	writer      *csv.Writer
	fields      []Field
	deviceCount int
	wroteHeader bool
}

func NewCSVEncoder(w io.Writer, fields []Field, deviceCount int) *CSVEncoder {
	return &CSVEncoder{
		writer:      csv.NewWriter(w),
		fields:      fields,
		deviceCount: deviceCount,
	}
}

func (e *CSVEncoder) Write(snapshot gpu.Snapshot) error {
	if !e.wroteHeader {
		if err := e.writer.Write(e.header()); err != nil {
			return err
		}
		e.wroteHeader = true
	}

	byIndex := make(map[int]gpu.DeviceSample, len(snapshot.Devices))
	for _, device := range snapshot.Devices {
		byIndex[device.Index] = device
	}

	row := []string{snapshot.Timestamp.UTC().Format(timeRFC3339), exportSource(snapshot.Source)}
	for i := 0; i < e.deviceCount; i++ {
		device, ok := byIndex[i]
		for _, field := range e.fields {
			if !ok {
				row = append(row, "")
				continue
			}
			row = append(row, field.Value(device).CSV)
		}
	}
	if err := e.writer.Write(row); err != nil {
		return err
	}
	e.writer.Flush()
	return e.writer.Error()
}

func (e *CSVEncoder) Flush() error {
	e.writer.Flush()
	return e.writer.Error()
}

func (e *CSVEncoder) header() []string {
	header := []string{"ts", "source"}
	for i := 0; i < e.deviceCount; i++ {
		for _, field := range e.fields {
			header = append(header, fmt.Sprintf("%s_gpu%d", field.Name, i))
		}
	}
	return header
}
