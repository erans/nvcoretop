package export

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"nvcoretop/internal/gpu"
)

var ErrUnknownField = errors.New("unknown export field")
var ErrDuplicateField = errors.New("duplicate export field")

type Field struct {
	Name  string
	Value func(gpu.DeviceSample) FieldValue
}

type FieldValue struct {
	JSON any
	CSV  string
}

var fieldRegistry = map[string]Field{
	"i": {Name: "i", Value: func(d gpu.DeviceSample) FieldValue {
		return intValue(d.Index)
	}},
	"name": {Name: "name", Value: func(d gpu.DeviceSample) FieldValue {
		return stringValue(d.Name)
	}},
	"uuid": {Name: "uuid", Value: func(d gpu.DeviceSample) FieldValue {
		return stringValue(d.UUID)
	}},
	"util": {Name: "util", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.GPUUtil)
	}},
	"mem_util": {Name: "mem_util", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.MemUtil)
	}},
	"mem_used": {Name: "mem_used", Value: func(d gpu.DeviceSample) FieldValue {
		return uint64Value(d.MemUsed)
	}},
	"mem_total": {Name: "mem_total", Value: func(d gpu.DeviceSample) FieldValue {
		return uint64Value(d.MemTotal)
	}},
	"temp": {Name: "temp", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.TempC)
	}},
	"power": {Name: "power", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.PowerW)
	}},
	"power_limit": {Name: "power_limit", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.PowerLimitW)
	}},
	"sm_clock": {Name: "sm_clock", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.SMClockMHz)
	}},
	"mem_clock": {Name: "mem_clock", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.MemClockMHz)
	}},
	"fan": {Name: "fan", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint32(d.FanPct)
	}},
	"proc_count": {Name: "proc_count", Value: func(d gpu.DeviceSample) FieldValue {
		return intValue(len(d.Processes))
	}},
	"proc_mem": {Name: "proc_mem", Value: func(d gpu.DeviceSample) FieldValue {
		var total uint64
		for _, proc := range d.Processes {
			total += proc.MemUsed
		}
		return uint64Value(total)
	}},
	"pcie_tx": {Name: "pcie_tx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.PCIeTxKBps)
	}},
	"pcie_rx": {Name: "pcie_rx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.PCIeRxKBps)
	}},
	"nvlink_tx": {Name: "nvlink_tx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.NVLinkTxKBps)
	}},
	"nvlink_rx": {Name: "nvlink_rx", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.NVLinkRxKBps)
	}},
	"ecc_sbe": {Name: "ecc_sbe", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.ECCSingleBit)
	}},
	"ecc_dbe": {Name: "ecc_dbe", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalUint64(d.ECCDoubleBit)
	}},
	"sm_active": {Name: "sm_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.SMActivePct)
	}},
	"tensor_active": {Name: "tensor_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.TensorActivePct)
	}},
	"dram_active": {Name: "dram_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.MemPipeActivePct)
	}},
	"mem_pipe_active": {Name: "mem_pipe_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.MemPipeActivePct)
	}},
	"fp32_active": {Name: "fp32_active", Value: func(d gpu.DeviceSample) FieldValue {
		return optionalFloat(d.FP32ActivePct)
	}},
}

var defaultFieldNames = []string{
	"i", "name", "uuid",
	"util", "mem_util", "mem_used", "mem_total", "temp",
	"power", "power_limit", "sm_clock", "mem_clock", "fan",
	"proc_count", "proc_mem",
	"pcie_tx", "pcie_rx", "nvlink_tx", "nvlink_rx",
	"ecc_sbe", "ecc_dbe",
	"sm_active", "tensor_active", "dram_active", "fp32_active",
}

func ResolveFields(names []string) ([]Field, error) {
	if len(names) == 0 {
		names = defaultFieldNames
	}

	fields := make([]Field, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		normalized := strings.TrimSpace(name)
		field, ok := fieldRegistry[normalized]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownField, normalized)
		}
		if _, exists := seen[normalized]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateField, normalized)
		}
		seen[normalized] = struct{}{}
		fields = append(fields, field)
	}
	return fields, nil
}

func intValue(value int) FieldValue {
	return FieldValue{JSON: value, CSV: strconv.Itoa(value)}
}

func uint64Value(value uint64) FieldValue {
	return FieldValue{JSON: value, CSV: strconv.FormatUint(value, 10)}
}

func stringValue(value string) FieldValue {
	return FieldValue{JSON: value, CSV: value}
}

func optionalUint32(value gpu.Optional[uint32]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatUint(uint64(value.Value), 10)}
}

func optionalUint64(value gpu.Optional[uint64]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatUint(value.Value, 10)}
}

func optionalFloat(value gpu.Optional[float64]) FieldValue {
	if !value.OK {
		return FieldValue{JSON: nil}
	}
	return FieldValue{JSON: value.Value, CSV: strconv.FormatFloat(value.Value, 'f', -1, 64)}
}
