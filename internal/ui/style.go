package ui

import "github.com/charmbracelet/lipgloss"

type palette struct {
	muted lipgloss.Style
	warn  lipgloss.Style
	hot   lipgloss.Style
	ok    lipgloss.Style
}

func styles(noColor bool) palette {
	if noColor {
		return palette{}
	}
	return palette{
		muted: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		warn:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		hot:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		ok:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
	}
}

func tempState(temp uint32) string {
	switch {
	case temp >= 80:
		return "hot"
	case temp >= 70:
		return "warn"
	default:
		return "ok"
	}
}

func utilState(util uint32) string {
	switch {
	case util >= 75:
		return "high"
	case util >= 25:
		return "active"
	default:
		return "idle"
	}
}

func powerNearLimit(power, limit float64) bool {
	return limit > 0 && power/limit >= 0.90
}
