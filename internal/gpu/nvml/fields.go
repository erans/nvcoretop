package nvml

import (
	"os"
	"strconv"
	"strings"
	"time"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
	"nvcoretop/internal/gpu"
)

const maxNVLinks = 12

type nvlinkTotals struct {
	at time.Time
	tx uint64
	rx uint64
}

func sampleDevice(index int, device nvidia.Device, processName func(uint32) string, previous *nvlinkTotals) gpu.DeviceSample {
	sample := gpu.DeviceSample{Index: index}

	if name, ret := device.GetName(); ok(ret) {
		sample.Name = name
	}
	if uuid, ret := device.GetUUID(); ok(ret) {
		sample.UUID = uuid
	}
	if mem, ret := device.GetMemoryInfo(); ok(ret) {
		sample.MemUsed = mem.Used
		sample.MemTotal = mem.Total
	}
	if util, ret := device.GetUtilizationRates(); ok(ret) {
		sample.GPUUtil = gpu.Some(util.Gpu)
		sample.MemUtil = gpu.Some(util.Memory)
	}
	if temp, ret := device.GetTemperature(nvidia.TEMPERATURE_GPU); ok(ret) {
		sample.TempC = gpu.Some(temp)
	}
	if milliWatts, ret := device.GetPowerUsage(); ok(ret) {
		sample.PowerW = gpu.Some(float64(milliWatts) / 1000)
	}
	if milliWatts, ret := device.GetPowerManagementLimit(); ok(ret) {
		sample.PowerLimitW = gpu.Some(float64(milliWatts) / 1000)
	}
	if clock, ret := device.GetClockInfo(nvidia.CLOCK_SM); ok(ret) {
		sample.SMClockMHz = gpu.Some(clock)
	}
	if clock, ret := device.GetClockInfo(nvidia.CLOCK_MEM); ok(ret) {
		sample.MemClockMHz = gpu.Some(clock)
	}
	if reasons, ret := device.GetCurrentClocksThrottleReasons(); ok(ret) {
		sample.ThrottleReasons = mapThrottleReasons(reasons)
	}
	if fan, ret := device.GetFanSpeed(); ok(ret) {
		sample.FanPct = gpu.Some(fan)
	}
	if processes, ret := device.GetComputeRunningProcesses(); ok(ret) {
		sample.Processes = mapProcesses(processes, processName)
	} else if ret == nvidia.ERROR_NO_PERMISSION {
		sample.ProcessLimited = true
	}
	if value, ret := device.GetPcieThroughput(nvidia.PCIE_UTIL_TX_BYTES); ok(ret) {
		sample.PCIeTxKBps = gpu.Some(uint64(value))
	}
	if value, ret := device.GetPcieThroughput(nvidia.PCIE_UTIL_RX_BYTES); ok(ret) {
		sample.PCIeRxKBps = gpu.Some(uint64(value))
	}
	if value, ret := device.GetTotalEccErrors(nvidia.MEMORY_ERROR_TYPE_CORRECTED, nvidia.VOLATILE_ECC); ok(ret) {
		sample.ECCSingleBit = gpu.Some(value)
	}
	if value, ret := device.GetTotalEccErrors(nvidia.MEMORY_ERROR_TYPE_UNCORRECTED, nvidia.VOLATILE_ECC); ok(ret) {
		sample.ECCDoubleBit = gpu.Some(value)
	}

	return sample
}

func mapProcesses(processes []nvidia.ProcessInfo, processName func(uint32) string) []gpu.Process {
	out := make([]gpu.Process, 0, len(processes))
	for _, process := range processes {
		out = append(out, gpu.Process{
			PID:     process.Pid,
			Name:    processName(process.Pid),
			MemUsed: process.UsedGpuMemory,
		})
	}
	return out
}

func processNameFromProc(pid uint32) string {
	data, err := os.ReadFile("/proc/" + strconv.FormatUint(uint64(pid), 10) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func mapThrottleReasons(bits uint64) gpu.ThrottleReasons {
	return gpu.ThrottleReasons{
		GPUIdle:            bits&nvidia.ClocksThrottleReasonGpuIdle != 0,
		ApplicationsClocks: bits&nvidia.ClocksThrottleReasonApplicationsClocksSetting != 0,
		SWPowerCap:         bits&nvidia.ClocksThrottleReasonSwPowerCap != 0,
		HWSlowdown:         bits&nvidia.ClocksThrottleReasonHwSlowdown != 0,
		SyncBoost:          bits&nvidia.ClocksThrottleReasonSyncBoost != 0,
		SWThermal:          bits&nvidia.ClocksThrottleReasonSwThermalSlowdown != 0,
		HWThermal:          bits&nvidia.ClocksThrottleReasonHwThermalSlowdown != 0,
		HWPowerBrake:       bits&nvidia.ClocksThrottleReasonHwPowerBrakeSlowdown != 0,
	}
}

func readNVLinkTotals(device nvidia.Device, at time.Time) (nvlinkTotals, bool) {
	totals := nvlinkTotals{at: at}
	found := false
	for link := 0; link < maxNVLinks; link++ {
		state, ret := device.GetNvLinkState(link)
		if !ok(ret) || state != nvidia.FEATURE_ENABLED {
			continue
		}
		rx, tx, ret := device.GetNvLinkUtilizationCounter(link, 0)
		if !ok(ret) {
			continue
		}
		totals.rx += rx
		totals.tx += tx
		found = true
	}
	return totals, found
}

func applyNVLinkDelta(sample *gpu.DeviceSample, previous, current nvlinkTotals) {
	elapsed := current.at.Sub(previous.at).Seconds()
	if elapsed <= 0 || current.tx < previous.tx || current.rx < previous.rx {
		return
	}
	sample.NVLinkTxKBps = gpu.Some(uint64((float64(current.tx-previous.tx) / 1024) / elapsed))
	sample.NVLinkRxKBps = gpu.Some(uint64((float64(current.rx-previous.rx) / 1024) / elapsed))
}
