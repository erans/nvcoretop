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
	heatHeight, includeContext := tensorWallLayout(lineBudget, m.height, len(devices))
	for i, device := range devices {
		block := renderTensorGPUBlockStyled(device, m.snapshot.Source, m.width, heatWidth, heatHeight, st, noColor, includeContext)
		separatorLines := 0
		if i > 0 {
			separatorLines = 1
		}
		neededLines := separatorLines + len(block)
		remainingDevices := len(devices) - i
		if !unlimited {
			availableLines := lineBudget - len(lines)
			if neededLines > availableLines {
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
	return renderTensorGPUBlockStyled(device, source, width, heatWidth, heatHeight, styles(true), true, true)
}

func renderTensorGPUBlockStyled(device gpu.DeviceSample, source gpu.Source, width, heatWidth, heatHeight int, st palette, noColor, includeContext bool) []string {
	name := truncateRunes(device.Name, tensorNameWidth(width))
	tensorText := tensorMetricSummary("Tensor Pipe", device.TensorActivePct)
	dramText := tensorMetricSummary("DRAM", device.MemPipeActivePct)
	header := renderBoundedStyledLine([]styledLinePart{
		{text: fmt.Sprintf("GPU %d %s  ", device.Index, name)},
		{
			text: tensorText,
			render: func(text string) string {
				return styleActivityText(text, device.TensorActivePct, st, noColor)
			},
		},
		{text: "  "},
		{
			text: dramText,
			render: func(text string) string {
				return styleActivityText(text, device.MemPipeActivePct, st, noColor)
			},
		},
	}, width, noColor)

	lines := []string{header}
	if includeContext {
		smText := percentFloatText(device.SMActivePct)
		fp32Text := percentFloatText(device.FP32ActivePct)
		utilText := percentText(device.GPUUtil)
		memText := memCell(device)
		tempText := tempCell(device)
		sourceText := source.String()
		context := renderBoundedStyledLine([]styledLinePart{
			{text: "  "},
			{text: "SM", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{
				text: smText,
				render: func(text string) string {
					return styleActivityText(text, device.SMActivePct, st, noColor)
				},
			},
			{text: "  "},
			{text: "FP32", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{
				text: fp32Text,
				render: func(text string) string {
					return styleActivityText(text, device.FP32ActivePct, st, noColor)
				},
			},
			{text: "  "},
			{text: "util", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{
				text: utilText,
				render: func(text string) string {
					return st.optionalActivity(percentFloat(device.GPUUtil), device.GPUUtil.OK).Render(text)
				},
			},
			{text: "  "},
			{text: "mem", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{text: memText, render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: "  "},
			{text: "temp", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{text: tempText, render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: "  "},
			{text: "source", render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{text: sourceText, render: func(text string) string { return styleMuted(text, st, noColor) }},
		}, width, noColor)
		lines = append(lines, context)
	}
	lines = appendActivityHeatmap(lines, "Tensor Pipe", device.TensorActivePct, width, heatWidth, heatHeight, st, noColor)
	lines = appendActivityHeatmap(lines, "DRAM", device.MemPipeActivePct, width, heatWidth, heatHeight, st, noColor)
	return lines
}

func tensorWallLayout(lineBudget, height, gpuCount int) (int, bool) {
	if lineBudget < 0 {
		return tensorHeatmapHeight(height, gpuCount), true
	}
	for heatHeight := maxTensorHeatmapHeight; heatHeight >= minTensorHeatmapHeight; heatHeight-- {
		if tensorWallLineNeed(gpuCount, heatHeight, true) <= lineBudget {
			return heatHeight, true
		}
	}
	for heatHeight := maxTensorHeatmapHeight; heatHeight >= minTensorHeatmapHeight; heatHeight-- {
		if tensorWallLineNeed(gpuCount, heatHeight, false) <= lineBudget {
			return heatHeight, false
		}
	}
	return minTensorHeatmapHeight, false
}

func tensorWallLineNeed(gpuCount, heatHeight int, includeContext bool) int {
	if gpuCount <= 0 {
		return 1
	}
	return 1 + gpuCount*tensorGPUBlockLineCount(heatHeight, includeContext) + gpuCount - 1
}

func tensorGPUBlockLineCount(heatHeight int, includeContext bool) int {
	if heatHeight < 1 {
		heatHeight = 1
	}
	lines := 1 + 2*(1+heatHeight)
	if includeContext {
		lines++
	}
	return lines
}

func appendActivityHeatmap(lines []string, label string, value gpu.Optional[float64], width, heatWidth, heatHeight int, st palette, noColor bool) []string {
	if !value.OK {
		line := renderBoundedStyledLine([]styledLinePart{
			{text: "  "},
			{text: label, render: func(text string) string { return styleMuted(text, st, noColor) }},
			{text: " "},
			{text: "unavailable (DCGM field missing)", render: func(text string) string { return styleMuted(text, st, noColor) }},
		}, width, noColor)
		return append(lines, line)
	}
	percent := tensorPercentText(value)
	summary := renderBoundedStyledLine([]styledLinePart{
		{text: "  "},
		{text: label, render: func(text string) string { return styleMuted(text, st, noColor) }},
		{text: " "},
		{text: percent, render: func(text string) string { return styleActivityText(text, value, st, noColor) }},
	}, width, noColor)
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

type styledLinePart struct {
	text   string
	render func(text string) string
}

func renderBoundedStyledLine(parts []styledLinePart, width int, noColor bool) string {
	var raw strings.Builder
	for _, part := range parts {
		raw.WriteString(part.text)
	}
	bounded := truncateRunes(raw.String(), width)
	if noColor {
		return bounded
	}

	prefix := bounded
	ellipsis := ""
	if width > 3 && bounded != raw.String() && strings.HasSuffix(bounded, "...") {
		prefix = strings.TrimSuffix(bounded, "...")
		ellipsis = "..."
	}

	var builder strings.Builder
	remaining := len(prefix)
	for _, part := range parts {
		if remaining <= 0 {
			break
		}
		text := part.text
		if len(text) > remaining {
			text = text[:remaining]
		}
		if part.render == nil {
			builder.WriteString(text)
		} else {
			builder.WriteString(part.render(text))
		}
		remaining -= len(text)
	}
	builder.WriteString(ellipsis)
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
	var run strings.Builder
	runKind := 0
	flush := func() {
		if run.Len() == 0 {
			return
		}
		switch runKind {
		case 1:
			builder.WriteString(cellStyle.Render(run.String()))
		case 2:
			builder.WriteString(st.muted.Render(run.String()))
		default:
			builder.WriteString(run.String())
		}
		run.Reset()
	}
	for _, r := range row {
		kind := 0
		switch r {
		case '█':
			kind = 1
		case '░':
			kind = 2
		}
		if kind != runKind {
			flush()
			runKind = kind
		}
		run.WriteRune(r)
	}
	flush()
	return builder.String()
}
