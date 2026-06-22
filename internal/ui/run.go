package ui

import (
	"context"
	"time"

	"nvcoretop/internal/gpu"

	tea "github.com/charmbracelet/bubbletea"
)

type sampleTickMsg struct{}

type runnerModel struct {
	Model
	ctx      context.Context
	sampler  gpu.Sampler
	interval time.Duration
}

func Run(ctx context.Context, sampler gpu.Sampler, interval time.Duration, options Options) error {
	model := runnerModel{
		Model:    NewModel(options),
		ctx:      ctx,
		sampler:  sampler,
		interval: interval,
	}
	program := tea.NewProgram(model, tea.WithAltScreen())
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			program.Quit()
		case <-done:
		}
	}()
	defer close(done)

	_, err := program.Run()
	return err
}

func (m runnerModel) Init() tea.Cmd {
	return sampleTickCmd(m.interval)
}

func (m runnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case sampleTickMsg:
		return m, tea.Batch(
			sampleCmd(m.ctx, m.sampler, m.paused),
			sampleTickCmd(m.interval),
		)
	default:
		next, cmd := updateModel(m.Model, msg)
		m.Model = next
		return m, cmd
	}
}

func sampleTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return sampleTickMsg{}
	})
}

func sampleCmd(ctx context.Context, sampler gpu.Sampler, paused bool) tea.Cmd {
	return func() tea.Msg {
		if paused {
			return nil
		}
		snapshot, err := sampler.Sample(ctx)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SnapshotMsg(snapshot)
	}
}
