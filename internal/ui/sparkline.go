package ui

import "strings"

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return "n/a"
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}

	min := values[0]
	max := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	var builder strings.Builder
	for _, value := range values {
		index := 0
		if max > min {
			index = int(((value - min) / (max - min)) * float64(len(sparkRunes)-1))
		}
		builder.WriteRune(sparkRunes[index])
	}
	return builder.String()
}
