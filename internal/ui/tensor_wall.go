package ui

import (
	"fmt"
	"math"
	"strings"

	"nvcoretop/internal/gpu"
)

const (
	minTensorHeatmapHeight = 1
	maxTensorHeatmapHeight = 6
)

func (m Model) renderTensorWall(lineBudget int) string {
	if lineBudget == 0 {
		return ""
	}
	devices := SortDevices(m.snapshot.Devices, m.sort)
	unlimited := lineBudget < 0
	lines := []string{truncateRunes("Tensor/DRAM Activity Wall", m.width)}
	if len(devices) == 0 {
		if !unlimited && len(lines) >= lineBudget {
			return strings.Join(lines, "\n")
		}
		lines = append(lines, truncateRunes("waiting for GPU samples...", m.width))
		return strings.Join(lines, "\n")
	}

	heatWidth := tensorHeatmapWidth(m.width)
	heatHeight := tensorHeatmapHeight(m.height, len(devices))
	for i, device := range devices {
		block := renderTensorGPUBlock(device, m.snapshot.Source, m.width, heatWidth, heatHeight)
		separatorLines := 0
		if i > 0 {
			separatorLines = 1
		}
		neededLines := separatorLines + len(block)
		remainingDevices := len(devices) - i
		if !unlimited {
			availableLines := lineBudget - len(lines)
			if neededLines > availableLines || (i > 0 && remainingDevices > 1 && neededLines == availableLines) {
				if availableLines > 0 {
					lines = append(lines, truncateRunes(fmt.Sprintf("... %d more GPU(s)", remainingDevices), m.width))
				}
				break
			}
		}
		if separatorLines > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return strings.Join(lines, "\n")
}

func renderTensorGPUBlock(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int) []string {
	name := truncateRunes(device.Name, tensorNameWidth(width))
	header := fmt.Sprintf("GPU %d %s  %s  %s",
		device.Index,
		name,
		tensorMetricSummary("Tensor Pipe", device.TensorActivePct),
		tensorMetricSummary("DRAM", device.MemPipeActivePct),
	)
	lines := []string{truncateRunes(header, width)}
	context := fmt.Sprintf("  SM %s  FP32 %s  util %s  mem %s  temp %s  source %s",
		percentFloatText(device.SMActivePct),
		percentFloatText(device.FP32ActivePct),
		percentText(device.GPUUtil),
		memCell(device),
		tempCell(device),
		source.String(),
	)
	lines = append(lines, truncateRunes(context, width))
	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight)
	return lines
}

func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int) []string {
	if !value.OK {
		return append(lines, truncateRunes(fmt.Sprintf("  %s unavailable (DCGM field missing)", label), width))
	}
	lines = append(lines, truncateRunes(fmt.Sprintf("  %s %s", label, tensorPercentText(value)), width))
	for _, row := range tensorHeatmapRows(value, heatWidth, heatHeight) {
		lines = append(lines, truncateRunes("  "+row, width))
	}
	return lines
}

func tensorHeatmapRows(value gpu.Optional[float64], width, height int) []string {
	if height < 1 {
		height = 1
	}
	if width < 1 {
		width = 1
	}

	percent := 0.0
	if value.OK {
		percent = clampFloat(value.Value, 0, 100)
	}
	filled := int(math.Round((percent / 100) * float64(width)))
	filled = clampInt(filled, 0, width)
	row := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	rows := make([]string, 0, height)
	for range height {
		rows = append(rows, row)
	}
	return rows
}

func tensorHeatmapWidth(width int) int {
	if width <= 0 {
		return 48
	}
	usable := width - 2
	if usable < 1 {
		return 1
	}
	return clampInt(usable, 1, 96)
}

func tensorHeatmapHeight(height, gpuCount int) int {
	if gpuCount <= 0 {
		return minTensorHeatmapHeight
	}
	perGPU := (height - 4) / gpuCount
	return clampInt((perGPU-3)/2, minTensorHeatmapHeight, maxTensorHeatmapHeight)
}

func tensorNameWidth(width int) int {
	if width <= 0 {
		return 24
	}
	return clampInt(width/4, 6, 32)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func tensorMetricSummary(label string, value gpu.Optional[float64]) string {
	if !value.OK {
		return label + " unavailable"
	}
	return fmt.Sprintf("%s %s", label, tensorPercentText(value))
}

func tensorPercentText(value gpu.Optional[float64]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", clampFloat(value.Value, 0, 100))
}
