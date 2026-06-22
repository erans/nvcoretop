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
	noColor := m.options.NoColor
	st := styles(noColor)
	devices := SortDevices(m.snapshot.Devices, m.sort)
	unlimited := lineBudget < 0
	title := truncateRunes("Tensor/DRAM Activity Wall", m.width)
	if !noColor {
		title = st.hot.Render(title)
	}
	lines := []string{title}
	if len(devices) == 0 {
		if !unlimited && len(lines) >= lineBudget {
			return strings.Join(lines, "\n")
		}
		lines = append(lines, styleMuted(truncateRunes("waiting for GPU samples...", m.width), st, noColor))
		return strings.Join(lines, "\n")
	}

	heatWidth := tensorHeatmapWidth(m.width)
	heatHeight := tensorHeatmapHeight(m.height, len(devices))
	for i, device := range devices {
		block := renderTensorGPUBlockStyled(device, m.snapshot.Source, m.width, heatWidth, heatHeight, st, noColor)
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
					overflow := truncateRunes(fmt.Sprintf("... %d more GPU(s)", remainingDevices), m.width)
					lines = append(lines, styleMuted(overflow, st, noColor))
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
	return renderTensorGPUBlockStyled(device, source, width, heatWidth, heatHeight, styles(true), true)
}

func renderTensorGPUBlockStyled(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int, st palette, noColor bool) []string {
	name := truncateRunes(device.Name, tensorNameWidth(width))
	tensorText := tensorMetricSummary("Tensor Pipe", device.TensorActivePct)
	dramText := tensorMetricSummary("DRAM", device.MemPipeActivePct)
	headerRaw := truncateRunes(fmt.Sprintf("GPU %d %s  %s  %s",
		device.Index,
		name,
		tensorText,
		dramText,
	), width)
	header := styleLineSegments(headerRaw, []styledSegment{
		{
			text: tensorText,
			render: func(text string) string {
				return styleActivityText(text, device.TensorActivePct, st, noColor)
			},
		},
		{
			text: dramText,
			render: func(text string) string {
				return styleActivityText(text, device.MemPipeActivePct, st, noColor)
			},
		},
	}, noColor)

	smText := percentFloatText(device.SMActivePct)
	fp32Text := percentFloatText(device.FP32ActivePct)
	utilText := percentText(device.GPUUtil)
	memText := memCell(device)
	tempText := tempCell(device)
	sourceText := source.String()
	contextRaw := truncateRunes(fmt.Sprintf("  SM %s  FP32 %s  util %s  mem %s  temp %s  source %s",
		smText,
		fp32Text,
		utilText,
		memText,
		tempText,
		sourceText,
	), width)
	context := styleLineSegments(contextRaw, []styledSegment{
		{text: "SM", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{
			text: smText,
			render: func(text string) string {
				return styleActivityText(text, device.SMActivePct, st, noColor)
			},
		},
		{text: "FP32", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{
			text: fp32Text,
			render: func(text string) string {
				return styleActivityText(text, device.FP32ActivePct, st, noColor)
			},
		},
		{text: "util", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{
			text: utilText,
			render: func(text string) string {
				return st.optionalActivity(percentFloat(device.GPUUtil), device.GPUUtil.OK).Render(text)
			},
		},
		{text: "mem", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: memText, render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: "temp", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: tempText, render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: "source", render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: sourceText, render: func(text string) string { return styleMuted(text, st, noColor) }},
	}, noColor)
	lines := []string{header, context}
	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight, st, noColor)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight, st, noColor)
	return lines
}

func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int, st palette, noColor bool) []string {
	if !value.OK {
		lineRaw := truncateRunes(fmt.Sprintf("  %s unavailable (DCGM field missing)", label), width)
		line := styleLineSegments(lineRaw, []styledSegment{
			{text: label, render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: "unavailable (DCGM field missing)", render: func(text string) string { return styleMuted(text, st, noColor) }},
		}, noColor)
		return append(lines, line)
	}
	percent := tensorPercentText(value)
	summaryRaw := truncateRunes(fmt.Sprintf("  %s %s", label, percent), width)
	summary := styleLineSegments(summaryRaw, []styledSegment{
		{text: label, render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: percent, render: func(text string) string { return styleActivityText(text, value, st, noColor) }},
	}, noColor)
	lines = append(lines, summary)
	for _, row := range tensorHeatmapRows(value, heatWidth, heatHeight) {
		line := truncateRunes("  "+row, width)
		if !noColor {
			line = styleHeatmapRow(line, value, st)
		}
		lines = append(lines, line)
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

type styledSegment struct {
	text   string
	render func(string) string
}

func styleLineSegments(line string, segments []styledSegment, noColor bool) string {
	if noColor {
		return line
	}
	var builder strings.Builder
	offset := 0
	for _, segment := range segments {
		if segment.text == "" {
			continue
		}
		index := strings.Index(line[offset:], segment.text)
		if index < 0 {
			continue
		}
		start := offset + index
		builder.WriteString(line[offset:start])
		builder.WriteString(segment.render(segment.text))
		offset = start + len(segment.text)
	}
	builder.WriteString(line[offset:])
	return builder.String()
}

func styleActivityText(text string, value gpu.Optional[float64], st palette, noColor bool) string {
	if noColor {
		return text
	}
	return st.optionalActivity(optionalFloatPercent(value), value.OK).Render(text)
}

func styleMuted(text string, st palette, noColor bool) string {
	if noColor {
		return text
	}
	return st.muted.Render(text)
}

func styleHeatmapRow(row string, value gpu.Optional[float64], st palette) string {
	cellStyle := st.muted
	if value.OK {
		cellStyle = st.activity(optionalFloatPercent(value))
	}
	var builder strings.Builder
	for _, r := range row {
		switch r {
		case '█':
			builder.WriteString(cellStyle.Render(string(r)))
		case '░':
			builder.WriteString(st.muted.Render(string(r)))
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
