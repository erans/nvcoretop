package nvml

import (
	"testing"

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

	got := sampleDevice(0, device, func(uint32) string { return "python" }, nil)
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

	got := sampleDevice(0, device, func(uint32) string { return "" }, nil)
	if !got.ProcessLimited {
		t.Fatalf("ProcessLimited = false, want true")
	}
}

func TestUnsupportedFieldsStayMissing(t *testing.T) {
	device := minimalMockDevice()
	device.GetTemperatureFunc = func(nvidia.TemperatureSensors) (uint32, nvidia.Return) {
		return 0, nvidia.ERROR_NOT_SUPPORTED
	}

	got := sampleDevice(0, device, func(uint32) string { return "" }, nil)
	if got.TempC.OK {
		t.Fatalf("TempC = %#v, want missing", got.TempC)
	}
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
	}
}

func assertOptional[T comparable](t *testing.T, name string, got gpu.Optional[T], want T) {
	t.Helper()
	if !got.OK || got.Value != want {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

var _ gpu.DeviceSample
