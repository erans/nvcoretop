package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"nvcoretop/internal/gpu"
)

type Format int

const (
	FormatJSONL Format = iota
	FormatCSV
)

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	ticker *time.Ticker
}

func (t realTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t realTicker) Stop() {
	t.ticker.Stop()
}

type Options struct {
	Format    Format
	Interval  time.Duration
	Duration  time.Duration
	Count     int
	Fields    []string
	NewTicker func(time.Duration) Ticker
}

func Run(ctx context.Context, sampler gpu.Sampler, w io.Writer, opts Options) (err error) {
	parentCtx := ctx
	if err := parentCtx.Err(); err != nil {
		return err
	}

	interval := opts.Interval
	if interval <= 0 {
		interval = time.Second
	}

	newTicker := opts.NewTicker
	if newTicker == nil {
		newTicker = func(d time.Duration) Ticker {
			return realTicker{ticker: time.NewTicker(d)}
		}
	}

	fields, err := ResolveFields(opts.Fields)
	if err != nil {
		return err
	}

	runCtx := parentCtx
	var cancel context.CancelFunc
	if opts.Duration > 0 {
		runCtx, cancel = context.WithTimeout(parentCtx, opts.Duration)
		defer cancel()
	}

	ticker := newTicker(interval)
	defer ticker.Stop()

	var csvEncoder *CSVEncoder
	switch opts.Format {
	case FormatJSONL:
	case FormatCSV:
		csvEncoder = NewCSVEncoder(w, fields, sampler.DeviceCount())
		defer func() {
			if err != nil {
				return
			}
			if flushErr := csvEncoder.Flush(); flushErr != nil {
				err = flushErr
			}
		}()
	default:
		return fmt.Errorf("unsupported export format: %d", opts.Format)
	}

	written := 0
	for {
		if runErr := runCtx.Err(); runErr != nil {
			if errors.Is(runErr, context.DeadlineExceeded) {
				return nil
			}
			if parentErr := parentCtx.Err(); parentErr != nil {
				return parentErr
			}
			return runErr
		}
		if opts.Count > 0 && written >= opts.Count {
			return nil
		}

		select {
		case <-runCtx.Done():
			if runErr := runCtx.Err(); runErr != nil {
				if errors.Is(runErr, context.DeadlineExceeded) {
					return nil
				}
				if parentErr := parentCtx.Err(); parentErr != nil {
					return parentErr
				}
				return runErr
			}
		case <-ticker.C():
			if runErr := runCtx.Err(); runErr != nil {
				if errors.Is(runErr, context.DeadlineExceeded) {
					return nil
				}
				if parentErr := parentCtx.Err(); parentErr != nil {
					return parentErr
				}
				return runErr
			}

			snapshot, sampleErr := sampler.Sample(runCtx)
			if sampleErr != nil {
				if runErr := runCtx.Err(); runErr != nil && errors.Is(runErr, context.DeadlineExceeded) {
					return nil
				}
				if parentErr := parentCtx.Err(); parentErr != nil {
					return parentErr
				}
				return sampleErr
			}

			switch opts.Format {
			case FormatJSONL:
				err = WriteJSONL(w, snapshot, fields)
			case FormatCSV:
				err = csvEncoder.Write(snapshot)
			default:
				err = fmt.Errorf("unsupported export format: %d", opts.Format)
			}
			if err != nil {
				return err
			}
			written++
		}
	}
}
