package ui

import (
	"fmt"
	"strings"

	"nvcoretop/internal/gpu"
)

const degradedWidth = 72

func (m Model) View() string {
	var parts []string
	degraded := m.width > 0 && m.width < degradedWidth
	if m.err != nil {
		parts = append(parts, "error: "+m.err.Error())
	}
	if degraded {
		parts = append(parts, m.renderDegraded())
	} else {
		parts = append(parts, m.renderOverview())
	}
	if m.detail && len(m.snapshot.Devices) > 0 {
		detail := m.renderDetail(m.selectedDevice())
		if degraded {
			detail = truncateLines(detail, m.width)
		}
		parts = append(parts, detail)
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
			overviewCoresCell(device, m.dcgmView || m.snapshot.Source == gpu.SourceNVMLDCGM),
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
		lines = append(lines,
			truncateRunes(fmt.Sprintf("GPU %d %s", device.Index, device.Name), m.width),
			truncateRunes(fmt.Sprintf("  util %s  mem %s", percentText(device.GPUUtil), memCell(device)), m.width),
			truncateRunes(fmt.Sprintf("  temp %s  pwr %s", tempCell(device), powerCell(device)), m.width),
		)
	}
	if len(lines) == 0 {
		return "waiting for GPU samples..."
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail(device gpu.DeviceSample) string {
	history, ok := m.history.Device(device.Index)
	utilHistory := "n/a"
	tempHistory := "n/a"
	powerHistory := "n/a"
	if ok {
		if history.Util != nil {
			utilHistory = Sparkline(history.Util.Values(), 32)
		}
		if history.Temp != nil {
			tempHistory = Sparkline(history.Temp.Values(), 32)
		}
		if history.Power != nil {
			powerHistory = Sparkline(history.Power.Values(), 32)
		}
	}
	lines := []string{
		fmt.Sprintf("Detail GPU %d %s", device.Index, device.Name),
		fmt.Sprintf("Util   %s", utilHistory),
		fmt.Sprintf("Temp   %s", tempHistory),
		fmt.Sprintf("Power  %s", powerHistory),
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
	return fmt.Sprintf("%s | interval %s | sort %s | source %s", status, m.options.Interval, m.sort.String(), m.snapshot.Source.String())
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

func overviewCoresCell(device gpu.DeviceSample, preferDCGM bool) string {
	if preferDCGM && device.SMActivePct.OK {
		return fmt.Sprintf("SM %s", percentFloatText(device.SMActivePct))
	}
	return CoresView(device, false)
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

func truncateRunes(value string, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func truncateLines(value string, width int) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = truncateRunes(line, width)
	}
	return strings.Join(lines, "\n")
}
