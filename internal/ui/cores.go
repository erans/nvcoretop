package ui

import (
	"fmt"
	"math"
	"strings"

	"nvcoretop/internal/gpu"
)

const (
	coreTileWidth  = 12
	coreTileHeight = 2
)

func CoresView(device gpu.DeviceSample, preferDCGM bool) string {
	if preferDCGM && device.SMActivePct.OK {
		leftTop := activityTile("SM", device.SMActivePct)
		rightTop := activityTile("Tensor", device.TensorActivePct)
		leftBottom := activityTile("MemPipe", device.MemPipeActivePct)
		rightBottom := activityTile("FP32", device.FP32ActivePct)

		lines := []string{"Core Activity"}
		lines = append(lines, combineTiles(leftTop, rightTop)...)
		lines = append(lines, combineTiles(leftBottom, rightBottom)...)
		return strings.Join(lines, "\n")
	}
	return fmt.Sprintf("cores %s %s", percentText(device.GPUUtil), bar(percentFloat(device.GPUUtil), 12))
}

func activityTile(label string, value gpu.Optional[float64]) []string {
	lines := []string{fmt.Sprintf("%s %s", label, percentFloatText(value))}
	for range coreTileHeight {
		lines = append(lines, tileRow(value))
	}
	return lines
}

func combineTiles(left, right []string) []string {
	lines := make([]string, 0, len(left))
	for i := range left {
		lines = append(lines, fmt.Sprintf("%-*s  %s", coreTileWidth, left[i], right[i]))
	}
	return lines
}

func tileRow(value gpu.Optional[float64]) string {
	filled := 0
	if value.OK {
		percent := optionalFloatPercent(value)
		filled = int(math.Round((percent / 100) * float64(coreTileWidth)))
		if filled < 0 {
			filled = 0
		}
		if filled > coreTileWidth {
			filled = coreTileWidth
		}
	}
	empty := coreTileWidth - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func bar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func percentText(value gpu.Optional[uint32]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%d%%", value.Value)
}

func percentFloat(value gpu.Optional[uint32]) float64 {
	if !value.OK {
		return 0
	}
	return float64(value.Value)
}

func percentFloatText(value gpu.Optional[float64]) string {
	if !value.OK {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", value.Value)
}

func optionalFloatPercent(value gpu.Optional[float64]) float64 {
	if !value.OK {
		return 0
	}
	return value.Value
}
