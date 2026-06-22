package ui

import (
	"fmt"
	"strings"

	"nvcoretop/internal/gpu"
)

func CoresView(device gpu.DeviceSample, preferDCGM bool) string {
	if preferDCGM && device.SMActivePct.OK {
		lines := []string{
			activityBar("SM", device.SMActivePct),
			activityBar("Tensor", device.TensorActivePct),
			activityBar("MemPipe", device.MemPipeActivePct),
			activityBar("FP32", device.FP32ActivePct),
		}
		return strings.Join(lines, "\n")
	}
	return fmt.Sprintf("cores %s %s", percentText(device.GPUUtil), bar(percentFloat(device.GPUUtil), 12))
}

func activityBar(label string, value gpu.Optional[float64]) string {
	return fmt.Sprintf("%-7s %6s %s", label, percentFloatText(value), bar(optionalFloatPercent(value), 16))
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
