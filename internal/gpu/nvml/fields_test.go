package nvml

import (
	"encoding/binary"
	"testing"
	"time"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock"
	"nvcoretop/internal/gpu"
)

func TestMapThrottleReasons(t *testing.T) {
	reasons := mapThrottleReasons(
		nvidia.ClocksThrottleReasonGpuIdle |
			nvidia.ClocksThrottleReasonSwPowerCap |
			nvidia.ClocksThrottleReasonHwThermalSlowdown,
	)
	if !reasons.GPUIdle || !reasons.SWPowerCap || !reasons.HWThermal {
		t.Fatalf("reasons = %#v", reasons)
	}
}

func TestSampleDeviceMapsSupportedFields(t *testing.T) {
	device := &mock.Device{
		GetNameFunc: func() (string, nvidia.Return) { return "RTX 3090", nvidia.SUCCESS },
		GetUUIDFunc: func() (string, nvidia.Return) { return "GPU-test", nvidia.SUCCESS },
		GetMemoryInfoFunc: func() (nvidia.Memory, nvidia.Return) {
			return nvidia.Memory{Used: 8, Total: 24}, nvidia.SUCCESS
		},
		GetUtilizationRatesFunc: func() (nvidia.Utilization, nvidia.Return) {
			return nvidia.Utilization{Gpu: 64, Memory: 12}, nvidia.SUCCESS
		},
		GetTemperatureFunc: func(sensor nvidia.TemperatureSensors) (uint32, nvidia.Return) {
			if sensor != nvidia.TEMPERATURE_GPU {
				t.Fatalf("temperature sensor = %v, want TEMPERATURE_GPU", sensor)
			}
			return 71, nvidia.SUCCESS
		},
		GetPowerUsageFunc:           func() (uint32, nvidia.Return) { return 285000, nvidia.SUCCESS },
		GetPowerManagementLimitFunc: func() (uint32, nvidia.Return) { return 350000, nvidia.SUCCESS },
		GetClockInfoFunc: func(clock nvidia.ClockType) (uint32, nvidia.Return) {
			switch clock {
			case nvidia.CLOCK_SM:
				return 1800, nvidia.SUCCESS
			case nvidia.CLOCK_MEM:
				return 9500, nvidia.SUCCESS
			default:
				t.Fatalf("clock = %v, want SM or MEM", clock)
				return 0, nvidia.ERROR_INVALID_ARGUMENT
			}
		},
		GetCurrentClocksThrottleReasonsFunc: func() (uint64, nvidia.Return) {
			return nvidia.ClocksThrottleReasonSwPowerCap, nvidia.SUCCESS
		},
		GetFanSpeedFunc: func() (uint32, nvidia.Return) { return 55, nvidia.SUCCESS },
		GetComputeRunningProcessesFunc: func() ([]nvidia.ProcessInfo, nvidia.Return) {
			return []nvidia.ProcessInfo{{Pid: 123, UsedGpuMemory: 4096}}, nvidia.SUCCESS
		},
		GetPcieThroughputFunc: func(counter nvidia.PcieUtilCounter) (uint32, nvidia.Return) {
			switch counter {
			case nvidia.PCIE_UTIL_TX_BYTES:
				return 111, nvidia.SUCCESS
			case nvidia.PCIE_UTIL_RX_BYTES:
				return 222, nvidia.SUCCESS
			default:
				t.Fatalf("PCIe counter = %v, want TX or RX", counter)
				return 0, nvidia.ERROR_INVALID_ARGUMENT
			}
		},
		GetTotalEccErrorsFunc: func(kind nvidia.MemoryErrorType, counter nvidia.EccCounterType) (uint64, nvidia.Return) {
			if counter != nvidia.VOLATILE_ECC {
				t.Fatalf("ECC counter = %v, want VOLATILE_ECC", counter)
			}
			switch kind {
			case nvidia.MEMORY_ERROR_TYPE_CORRECTED:
				return 1, nvidia.SUCCESS
			case nvidia.MEMORY_ERROR_TYPE_UNCORRECTED:
				return 2, nvidia.SUCCESS
			default:
				t.Fatalf("ECC kind = %v, want corrected or uncorrected", kind)
				return 0, nvidia.ERROR_INVALID_ARGUMENT
			}
		},
	}

	got := sampleDevice(0, device, func(uint32) string { return "python" })
	if got.Index != 0 || got.Name != "RTX 3090" || got.UUID != "GPU-test" {
		t.Fatalf("identity = %#v", got)
	}
	if got.MemUsed != 8 || got.MemTotal != 24 {
		t.Fatalf("memory = used %d total %d, want 8/24", got.MemUsed, got.MemTotal)
	}
	assertOptional(t, "GPUUtil", got.GPUUtil, 64)
	assertOptional(t, "MemUtil", got.MemUtil, 12)
	assertOptional(t, "TempC", got.TempC, 71)
	assertOptional(t, "PowerW", got.PowerW, 285.0)
	assertOptional(t, "PowerLimitW", got.PowerLimitW, 350.0)
	assertOptional(t, "SMClockMHz", got.SMClockMHz, 1800)
	assertOptional(t, "MemClockMHz", got.MemClockMHz, 9500)
	assertOptional(t, "FanPct", got.FanPct, 55)
	assertOptional(t, "PCIeTxKBps", got.PCIeTxKBps, uint64(111))
	assertOptional(t, "PCIeRxKBps", got.PCIeRxKBps, uint64(222))
	assertOptional(t, "ECCSingleBit", got.ECCSingleBit, uint64(1))
	assertOptional(t, "ECCDoubleBit", got.ECCDoubleBit, uint64(2))
	if !got.ThrottleReasons.SWPowerCap {
		t.Fatalf("missing throttle reason: %#v", got.ThrottleReasons)
	}
	if len(got.Processes) != 1 {
		t.Fatalf("process count = %d, want 1: %#v", len(got.Processes), got.Processes)
	}
	if got.Processes[0].PID != 123 || got.Processes[0].Name != "python" || got.Processes[0].MemUsed != 4096 {
		t.Fatalf("process = %#v", got.Processes[0])
	}
	if got.ProcessLimited {
		t.Fatalf("ProcessLimited = true, want false")
	}
}

func TestSampleDeviceMarksPermissionLimitedProcesses(t *testing.T) {
	device := minimalMockDevice()
	device.GetComputeRunningProcessesFunc = func() ([]nvidia.ProcessInfo, nvidia.Return) {
		return nil, nvidia.ERROR_NO_PERMISSION
	}

	got := sampleDevice(0, device, func(uint32) string { return "" })
	if !got.ProcessLimited {
		t.Fatalf("ProcessLimited = false, want true")
	}
}

func TestUnsupportedFieldsStayMissing(t *testing.T) {
	device := minimalMockDevice()
	device.GetTemperatureFunc = func(nvidia.TemperatureSensors) (uint32, nvidia.Return) {
		return 0, nvidia.ERROR_NOT_SUPPORTED
	}

	got := sampleDevice(0, device, func(uint32) string { return "" })
	if got.TempC.OK {
		t.Fatalf("TempC = %#v, want missing", got.TempC)
	}
}

func TestApplyNVLinkDelta(t *testing.T) {
	previous := nvlinkTotals{at: time.Unix(10, 0), tx: 1024, rx: 2048}
	current := nvlinkTotals{at: time.Unix(12, 0), tx: 3072, rx: 6144}
	sample := gpu.DeviceSample{Index: 0}

	applyNVLinkDelta(&sample, previous, current)

	if !sample.NVLinkTxKBps.OK || sample.NVLinkTxKBps.Value != 1 {
		t.Fatalf("NVLinkTxKBps = %#v, want 1", sample.NVLinkTxKBps)
	}
	if !sample.NVLinkRxKBps.OK || sample.NVLinkRxKBps.Value != 2 {
		t.Fatalf("NVLinkRxKBps = %#v, want 2", sample.NVLinkRxKBps)
	}
}

func TestApplyNVLinkDeltaSkipsInvalidDeltas(t *testing.T) {
	tests := []struct {
		name     string
		previous nvlinkTotals
		current  nvlinkTotals
	}{
		{
			name:     "zero elapsed",
			previous: nvlinkTotals{at: time.Unix(10, 0), tx: 1024, rx: 1024},
			current:  nvlinkTotals{at: time.Unix(10, 0), tx: 2048, rx: 2048},
		},
		{
			name:     "negative elapsed",
			previous: nvlinkTotals{at: time.Unix(10, 0), tx: 1024, rx: 1024},
			current:  nvlinkTotals{at: time.Unix(9, 0), tx: 2048, rx: 2048},
		},
		{
			name:     "tx rolls backward",
			previous: nvlinkTotals{at: time.Unix(10, 0), tx: 2048, rx: 1024},
			current:  nvlinkTotals{at: time.Unix(12, 0), tx: 1024, rx: 2048},
		},
		{
			name:     "rx rolls backward",
			previous: nvlinkTotals{at: time.Unix(10, 0), tx: 1024, rx: 2048},
			current:  nvlinkTotals{at: time.Unix(12, 0), tx: 2048, rx: 1024},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := gpu.DeviceSample{Index: 0}

			applyNVLinkDelta(&sample, tt.previous, tt.current)

			if sample.NVLinkTxKBps.OK || sample.NVLinkRxKBps.OK {
				t.Fatalf("NVLink throughput = tx %#v rx %#v, want missing", sample.NVLinkTxKBps, sample.NVLinkRxKBps)
			}
		})
	}
}

func TestReadNVLinkTotalsUsesFieldValuesForEnabledLinks(t *testing.T) {
	device := minimalMockDevice()
	enabledLinks := map[int]struct {
		tx uint64
		rx uint64
	}{
		1: {tx: 1024, rx: 2048},
		3: {tx: 3072, rx: 4096},
	}
	device.GetNvLinkStateFunc = func(link int) (nvidia.EnableState, nvidia.Return) {
		if _, ok := enabledLinks[link]; ok {
			return nvidia.FEATURE_ENABLED, nvidia.SUCCESS
		}
		return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
	}
	var requests [][]nvidia.FieldValue
	device.GetFieldValuesFunc = func(values []nvidia.FieldValue) nvidia.Return {
		requests = append(requests, append([]nvidia.FieldValue(nil), values...))
		if len(values) != 2 {
			t.Fatalf("field value count = %d, want 2", len(values))
		}
		link := int(values[0].ScopeId)
		counters, ok := enabledLinks[link]
		if !ok {
			t.Fatalf("field values requested for disabled link %d", link)
		}
		setUnsignedFieldValue(&values[0], counters.tx, nvidia.SUCCESS)
		setUnsignedFieldValue(&values[1], counters.rx, nvidia.SUCCESS)
		return nvidia.SUCCESS
	}

	totals, found := readNVLinkTotals(device, time.Unix(20, 0))

	if !found {
		t.Fatalf("found = false, want true")
	}
	if totals.tx != 4096 || totals.rx != 6144 {
		t.Fatalf("totals = tx %d rx %d, want tx 4096 rx 6144", totals.tx, totals.rx)
	}
	if len(requests) != 2 {
		t.Fatalf("field value request count = %d, want 2", len(requests))
	}
	assertNVLinkFieldRequest(t, requests[0], 1)
	assertNVLinkFieldRequest(t, requests[1], 3)
}

func TestReadNVLinkTotalsRejectsPartialFieldFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]nvidia.FieldValue) nvidia.Return
	}{
		{
			name: "field values call fails",
			mutate: func([]nvidia.FieldValue) nvidia.Return {
				return nvidia.ERROR_UNKNOWN
			},
		},
		{
			name: "tx field fails",
			mutate: func(values []nvidia.FieldValue) nvidia.Return {
				setUnsignedFieldValue(&values[0], 1024, nvidia.ERROR_UNKNOWN)
				setUnsignedFieldValue(&values[1], 2048, nvidia.SUCCESS)
				return nvidia.SUCCESS
			},
		},
		{
			name: "rx field has unsupported value type",
			mutate: func(values []nvidia.FieldValue) nvidia.Return {
				setUnsignedFieldValue(&values[0], 1024, nvidia.SUCCESS)
				setUnsignedFieldValue(&values[1], 2048, nvidia.SUCCESS)
				values[1].ValueType = uint32(nvidia.VALUE_TYPE_SIGNED_INT)
				return nvidia.SUCCESS
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := minimalMockDevice()
			device.GetNvLinkStateFunc = func(link int) (nvidia.EnableState, nvidia.Return) {
				if link == 0 {
					return nvidia.FEATURE_ENABLED, nvidia.SUCCESS
				}
				return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
			}
			device.GetFieldValuesFunc = tt.mutate

			totals, found := readNVLinkTotals(device, time.Unix(20, 0))

			if found {
				t.Fatalf("found = true, want false with totals %#v", totals)
			}
		})
	}
}

func TestReadNVLinkTotalsIncludesMaxNVLinkBoundary(t *testing.T) {
	lastLink := nvidia.NVLINK_MAX_LINKS - 1
	device := minimalMockDevice()
	var stateCalls []int
	device.GetNvLinkStateFunc = func(link int) (nvidia.EnableState, nvidia.Return) {
		stateCalls = append(stateCalls, link)
		if link == lastLink {
			return nvidia.FEATURE_ENABLED, nvidia.SUCCESS
		}
		return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
	}
	var request []nvidia.FieldValue
	device.GetFieldValuesFunc = func(values []nvidia.FieldValue) nvidia.Return {
		request = append([]nvidia.FieldValue(nil), values...)
		setUnsignedFieldValue(&values[0], 1024, nvidia.SUCCESS)
		setUnsignedFieldValue(&values[1], 2048, nvidia.SUCCESS)
		return nvidia.SUCCESS
	}

	totals, found := readNVLinkTotals(device, time.Unix(20, 0))

	if !found {
		t.Fatalf("found = false, want true")
	}
	if totals.tx != 1024 || totals.rx != 2048 {
		t.Fatalf("totals = tx %d rx %d, want tx 1024 rx 2048", totals.tx, totals.rx)
	}
	if len(stateCalls) != nvidia.NVLINK_MAX_LINKS {
		t.Fatalf("state call count = %d, want %d", len(stateCalls), nvidia.NVLINK_MAX_LINKS)
	}
	if stateCalls[len(stateCalls)-1] != lastLink {
		t.Fatalf("last state call = %d, want %d", stateCalls[len(stateCalls)-1], lastLink)
	}
	assertNVLinkFieldRequest(t, request, lastLink)
}

func minimalMockDevice() *mock.Device {
	return &mock.Device{
		GetNameFunc:             func() (string, nvidia.Return) { return "GPU", nvidia.SUCCESS },
		GetUUIDFunc:             func() (string, nvidia.Return) { return "uuid", nvidia.SUCCESS },
		GetMemoryInfoFunc:       func() (nvidia.Memory, nvidia.Return) { return nvidia.Memory{}, nvidia.SUCCESS },
		GetUtilizationRatesFunc: func() (nvidia.Utilization, nvidia.Return) { return nvidia.Utilization{}, nvidia.SUCCESS },
		GetTemperatureFunc: func(nvidia.TemperatureSensors) (uint32, nvidia.Return) {
			return 0, nvidia.SUCCESS
		},
		GetPowerUsageFunc:           func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetPowerManagementLimitFunc: func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetClockInfoFunc:            func(nvidia.ClockType) (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetCurrentClocksThrottleReasonsFunc: func() (uint64, nvidia.Return) {
			return 0, nvidia.SUCCESS
		},
		GetFanSpeedFunc: func() (uint32, nvidia.Return) { return 0, nvidia.ERROR_NOT_SUPPORTED },
		GetComputeRunningProcessesFunc: func() ([]nvidia.ProcessInfo, nvidia.Return) {
			return nil, nvidia.SUCCESS
		},
		GetPcieThroughputFunc: func(nvidia.PcieUtilCounter) (uint32, nvidia.Return) {
			return 0, nvidia.ERROR_NOT_SUPPORTED
		},
		GetTotalEccErrorsFunc: func(nvidia.MemoryErrorType, nvidia.EccCounterType) (uint64, nvidia.Return) {
			return 0, nvidia.ERROR_NOT_SUPPORTED
		},
		GetNvLinkStateFunc: func(int) (nvidia.EnableState, nvidia.Return) {
			return nvidia.FEATURE_DISABLED, nvidia.SUCCESS
		},
	}
}

func assertNVLinkFieldRequest(t *testing.T, got []nvidia.FieldValue, link int) {
	t.Helper()
	if len(got) != 2 {
		t.Fatalf("field value count = %d, want 2", len(got))
	}
	if got[0].FieldId != nvidia.FI_DEV_NVLINK_COUNT_XMIT_BYTES || got[0].ScopeId != uint32(link) {
		t.Fatalf("tx field request = %#v, want field %d scope %d", got[0], nvidia.FI_DEV_NVLINK_COUNT_XMIT_BYTES, link)
	}
	if got[1].FieldId != nvidia.FI_DEV_NVLINK_COUNT_RCV_BYTES || got[1].ScopeId != uint32(link) {
		t.Fatalf("rx field request = %#v, want field %d scope %d", got[1], nvidia.FI_DEV_NVLINK_COUNT_RCV_BYTES, link)
	}
}

func setUnsignedFieldValue(value *nvidia.FieldValue, counter uint64, ret nvidia.Return) {
	value.ValueType = uint32(nvidia.VALUE_TYPE_UNSIGNED_LONG_LONG)
	value.NvmlReturn = uint32(ret)
	binary.LittleEndian.PutUint64(value.Value[:], counter)
}

func assertOptional[T comparable](t *testing.T, name string, got gpu.Optional[T], want T) {
	t.Helper()
	if !got.OK || got.Value != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

var _ gpu.DeviceSample
