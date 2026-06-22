package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"nvcoretop/internal/app"
	"nvcoretop/internal/export"
	"nvcoretop/internal/gpu"
	"nvcoretop/internal/ui"
)

var version = "dev"

var runTUI = ui.Run

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

		sampler := gpu.NewFakeSampler([]gpu.FakeStep{{
			Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
		}})

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

		return export.Run(context.Background(), sampler, writer, export.Options{
			Format:   format,
			Interval: cfg.Interval,
			Duration: cfg.Duration,
			Count:    cfg.Count,
			Fields:   cfg.Fields,
		})
	default:
		sampler := gpu.NewFakeSampler([]gpu.FakeStep{{
			Snapshot: gpu.Snapshot{Source: gpu.SourceNVML},
		}})
		defer sampler.Close()
		return runTUI(context.Background(), sampler, cfg.Interval, ui.Options{
			Interval:      cfg.Interval.String(),
			NoColor:       cfg.NoColor,
			ForceDCGMView: cfg.DCGM,
		})
	}
}

func helpText() string {
	return fmt.Sprintf(`nvcoretop %s

Usage:
  nvcoretop [--json|--csv] [--output FILE] [--interval DURATION] [--duration DURATION] [--count N] [--fields LIST]
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
`, version)
}
