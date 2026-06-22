package ui

import (
	"fmt"
	"strings"

	"nvcoretop/internal/gpu"
)

const degradedWidth = 72

func (m Model) View() string {
	var parts []string
	if m.err != nil {
		parts = append(parts, "error: "+m.err.Error())
	}
	if m.width > 0 && m.width < degradedWidth {
		parts = append(parts, m.renderDegraded())
	} else {
		parts = append(parts, m.renderOverview())
	}
	if m.detail && len(m.snapshot.Devices) > 0 {
		parts = append(parts, m.renderDetail(m.selectedDevice()))
	}
	parts = append(parts, m.renderFooter())
	if m.help {
		parts = append(parts, "keys: up/down/j/k select | enter/tab detail | s sort | d dcgm | p pause | ? help | q quit")
	}
	return strings.Join(parts, "\n")
}

func (m Model) selectedDevice() gpu.DeviceSample {
	devices := SortDevices(m.snapshot.Devices, m.sort)
	if len(devices) == 0 {
		return gpu.DeviceSample{}
	}
	if m.selected >= len(devices) {
		return devices[len(devices)-1]
	}
	return devices[m.selected]
}

func (m Model) renderOverview() string {
	lines := []string{" #  NAME        UTIL        MEM             TEMP   PWR        CORES"}
	for row, device := range SortDevices(m.snapshot.Devices, m.sort) {
		cursor := " "
		if row == m.selected {
			cursor = ">"
		}
		lines = append(lines, fmt.Sprintf("%s%2d  %-10.10s %-10s %-14s %-6s %-10s %s",
			cursor,
			device.Index,
			device.Name,
			utilCell(device),
			memCell(device),
			tempCell(device),
			powerCell(device),
			CoresView(device, m.dcgmView || m.snapshot.Source == gpu.SourceNVMLDCGM),
		))
	}
	if len(m.snapshot.Devices) == 0 {
		lines = append(lines, "waiting for GPU samples...")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDegraded() string {
	lines := make([]string, 0, len(m.snapshot.Devices))
	for _, device := range SortDevices(m.snapshot.Devices, m.sort) {
		lines = append(lines, fmt.Sprintf("GPU %d %s  util %s  mem %s  temp %s  pwr %s",
			device.Index,
			device.Name,
			percentText(device.GPUUtil),
			memCell(device),
			tempCell(device),
			powerCell(device),
		))
	}
	if len(lines) == 0 {
		return "waiting for GPU samples..."
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail(device gpu.DeviceSample) string {
	history, _ := m.history.Device(device.Index)
	lines := []string{
		fmt.Sprintf("Detail GPU %d %s", device.Index, device.Name),
		fmt.Sprintf("Util   %s", Sparkline(history.Util.Values(), 32)),
		fmt.Sprintf("Temp   %s", Sparkline(history.Temp.Values(), 32)),
		fmt.Sprintf("Power  %s", Sparkline(history.Power.Values(), 32)),
		fmt.Sprintf("Clocks SM %s MHz  MEM %s MHz", optionalUint(device.SMClockMHz), optionalUint(device.MemClockMHz)),
		fmt.Sprintf("Throttle %s", throttleText(device.ThrottleReasons)),
		fmt.Sprintf("Fan %s", optionalUint(device.FanPct)),
		"Processes",
		processTable(device),
		fmt.Sprintf("PCIe tx %s KB/s rx %s KB/s", optionalUint64Text(device.PCIeTxKBps), optionalUint64Text(device.PCIeRxKBps)),
		fmt.Sprintf("NVLink tx %s KB/s rx %s KB/s", optionalUint64Text(device.NVLinkTxKBps), optionalUint64Text(device.NVLinkRxKBps)),
		fmt.Sprintf("ECC single %s double %s", optionalUint64Text(device.ECCSingleBit), optionalUint64Text(device.ECCDoubleBit)),
		CoresView(device, m.dcgmView || m.snapshot.Source == gpu.SourceNVMLDCGM),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFooter() string {
	status := "running"
	if m.paused {
		status = "paused"
	}
	return fmt.Sprintf("%s | sort %s | source %s", status, m.sort.String(), m.snapshot.Source.String())
}

func utilCell(device gpu.DeviceSample) string {
	if !device.GPUUtil.OK {
		return "n/a"
	}
	return fmt.Sprintf("%s %3d%%", bar(float64(device.GPUUtil.Value), 6), device.GPUUtil.Value)
}

func memCell(device gpu.DeviceSample) string {
	if device.MemTotal == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f/%.1f GB", bytesToGB(device.MemUsed), bytesToGB(device.MemTotal))
}

func tempCell(device gpu.DeviceSample) string {
	if !device.TempC.OK {
		return "n/a"
	}
	marker := ""
	if device.ThrottleReasons.Active() {
		marker = "^"
	}
	return fmt.Sprintf("%dC%s", device.TempC.Value, marker)
}

func powerCell(device gpu.DeviceSample) string {
	if !device.PowerW.OK {
		return "n/a"
	}
	if device.PowerLimitW.OK {
		return fmt.Sprintf("%.0f/%.0fW", device.PowerW.Value, device.PowerLimitW.Value)
	}
	return fmt.Sprintf("%.0fW", device.PowerW.Value)
}

func optionalUint(value gpu.Optional[uint32]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d", value.Value)
}

func optionalUint64Text(value gpu.Optional[uint64]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d", value.Value)
}

func throttleText(reasons gpu.ThrottleReasons) string {
	if !reasons.Active() {
		return "none"
	}
	return strings.Join(reasons.Names(), ",")
}

func processTable(device gpu.DeviceSample) string {
	if device.ProcessLimited {
		return "process query permission-limited"
	}
	if len(device.Processes) == 0 {
		return "no processes"
	}
	lines := []string{"PID      NAME                  VRAM"}
	for _, proc := range device.Processes {
		lines = append(lines, fmt.Sprintf("%-8d %-20.20s %.1f GB", proc.PID, proc.Name, bytesToGB(proc.MemUsed)))
	}
	return strings.Join(lines, "\n")
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}
