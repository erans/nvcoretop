package ui

import (
	"nvcoretop/internal/gpu"
	"nvcoretop/internal/history"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultHistoryWindow = 120

type Options struct {
	Interval      string
	NoColor       bool
	HistoryWindow int
	ForceDCGMView bool
}

type SnapshotMsg gpu.Snapshot

type ErrMsg struct {
	Err error
}

type viewMode int

const (
	viewOverview viewMode = iota
	viewTensorWall
)

type Model struct {
	snapshot gpu.Snapshot
	history  *history.Store
	selected int
	detail   bool
	paused   bool
	help     bool
	dcgmView bool
	sort     SortMode
	view     viewMode
	width    int
	height   int
	err      error
	options  Options
}

func NewModel(options Options) Model {
	window := options.HistoryWindow
	if window <= 0 {
		window = defaultHistoryWindow
	}
	options.HistoryWindow = window
	return Model{
		history:  history.NewStore(window),
		dcgmView: options.ForceDCGMView,
		sort:     SortIndex,
		options:  options,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := updateModel(m, msg)
	return next, cmd
}

func updateModel(m Model, msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case SnapshotMsg:
		m.snapshot = gpu.Snapshot(msg)
		m.history.Add(m.snapshot)
		m.err = nil
		m.clampSelection()
	case ErrMsg:
		m.err = msg.Err
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			m.selected++
			m.clampSelection()
		case "up", "k":
			m.selected--
			m.clampSelection()
		case "enter", "tab":
			m.detail = !m.detail
		case "s":
			m.sort = m.sort.Next()
			m.clampSelection()
		case "d":
			m.dcgmView = !m.dcgmView
		case "t":
			if m.view == viewTensorWall {
				m.view = viewOverview
			} else {
				m.view = viewTensorWall
			}
		case "o":
			m.view = viewOverview
			m.detail = false
		case "p":
			m.paused = !m.paused
		case "?":
			m.help = !m.help
		}
	}
	return m, nil
}

func (m *Model) clampSelection() {
	count := len(m.snapshot.Devices)
	if count == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= count {
		m.selected = count - 1
	}
}
