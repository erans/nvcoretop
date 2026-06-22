package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type palette struct {
	muted lipgloss.Style
	warn  lipgloss.Style
	hot   lipgloss.Style
	ok    lipgloss.Style
}

var ansiRenderer = func() *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI256)
	return renderer
}()

func styles(noColor bool) palette {
	if noColor {
		return palette{}
	}
	return palette{
		muted: lipgloss.NewStyle().Renderer(ansiRenderer).Foreground(lipgloss.Color("245")),
		warn:  lipgloss.NewStyle().Renderer(ansiRenderer).Foreground(lipgloss.Color("214")),
		hot:   lipgloss.NewStyle().Renderer(ansiRenderer).Foreground(lipgloss.Color("196")),
		ok:    lipgloss.NewStyle().Renderer(ansiRenderer).Foreground(lipgloss.Color("42")),
	}
}

func (p palette) activity(percent float64) lipgloss.Style {
	switch {
	case percent >= 75:
		return p.hot
	case percent >= 50:
		return p.warn
	case percent > 0:
		return p.ok
	default:
		return p.muted
	}
}

func (p palette) optionalActivity(value float64, ok bool) lipgloss.Style {
	if !ok {
		return p.muted
	}
	return p.activity(value)
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
