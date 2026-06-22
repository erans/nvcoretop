package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"nvcoretop/internal/app"
	"nvcoretop/internal/export"
	"nvcoretop/internal/sampler"
	"nvcoretop/internal/ui"
)

var version = "dev"

var runTUI = ui.Run
var createSampler = sampler.New
var newRuntimeContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) (err error) {
	cfg, err := app.ParseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprint(stdout, helpText())
			return writeErr
		}
		return err
	}

	if cfg.Version {
		_, err := fmt.Fprintf(stdout, "nvcoretop %s\n", version)
		return err
	}

	switch cfg.Mode {
	case app.ModeJSON, app.ModeCSV:
		format := export.FormatJSONL
		if cfg.Mode == app.ModeCSV {
			format = export.FormatCSV
		}

		runCtx, stop := newRuntimeContext()
		defer stop()

		created, createErr := createSampler(sampler.Options{ForceDCGM: cfg.DCGM})
		if createErr != nil {
			return createErr
		}
		defer func() {
			if closeErr := created.Sampler.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
		}()
		if created.Notice != "" {
			if _, printErr := fmt.Fprintln(stderr, created.Notice); printErr != nil {
				return printErr
			}
		}

		writer := stdout
		if cfg.Output != "-" {
			file, createErr := os.Create(cfg.Output)
			if createErr != nil {
				return createErr
			}
			defer func() {
				if closeErr := file.Close(); err == nil && closeErr != nil {
					err = closeErr
				}
			}()
			writer = file
		}

		return cleanRuntimeCancel(runCtx, export.Run(runCtx, created.Sampler, writer, export.Options{
			Format:   format,
			Interval: cfg.Interval,
			Duration: cfg.Duration,
			Count:    cfg.Count,
			Fields:   cfg.Fields,
		}))
	default:
		runCtx, stop := newRuntimeContext()
		defer stop()

		created, createErr := createSampler(sampler.Options{ForceDCGM: cfg.DCGM})
		if createErr != nil {
			return createErr
		}
		defer func() {
			if closeErr := created.Sampler.Close(); err == nil && closeErr != nil {
				err = closeErr
			}
		}()
		if created.Notice != "" {
			if _, printErr := fmt.Fprintln(stderr, created.Notice); printErr != nil {
				return printErr
			}
		}
		return cleanRuntimeCancel(runCtx, runTUI(runCtx, created.Sampler, cfg.Interval, ui.Options{
			Interval:      cfg.Interval.String(),
			NoColor:       cfg.NoColor,
			ForceDCGMView: cfg.DCGM,
		}))
	}
}

func cleanRuntimeCancel(ctx context.Context, err error) error {
	if err != nil && errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func helpText() string {
	return fmt.Sprintf(`nvcoretop %s

Usage:
  nvcoretop [--json|--csv] [--output FILE] [--interval DURATION] [--duration DURATION] [--count N] [--fields LIST] [--dcgm] [--no-color]
  nvcoretop --version
  nvcoretop --help

Options:
  --json          stream JSONL export output
  --csv           stream CSV export output
  --output        destination file, or - for stdout
  --interval      sampling interval
  --duration      stop after the given duration
  --count         stop after N samples
  --fields        comma-separated export fields
  --dcgm          force DCGM activity
  --no-color      disable colored output
`, version)
}
