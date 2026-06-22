package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrMutuallyExclusiveFormat = errors.New("--json and --csv are mutually exclusive")
	ErrInvalidInterval         = errors.New("--interval must be positive")
	ErrInvalidCount            = errors.New("--count cannot be negative")
	ErrInvalidDuration         = errors.New("--duration cannot be negative")
	ErrInvalidFields           = errors.New("--fields contains an empty entry")
	ErrUnexpectedArgs          = errors.New("unexpected positional arguments")
)

type Mode int

const (
	ModeTUI Mode = iota
	ModeJSON
	ModeCSV
)

type Config struct {
	Mode     Mode
	Interval time.Duration
	Output   string
	Duration time.Duration
	Count    int
	Fields   []string
	DCGM     bool
	NoColor  bool
	Version  bool
}

func ParseArgs(args []string) (Config, error) {
	cfg := Config{
		Mode:     ModeTUI,
		Interval: time.Second,
		Output:   "-",
	}

	flags := flag.NewFlagSet("nvcoretop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonMode := flags.Bool("json", false, "stream JSONL")
	csvMode := flags.Bool("csv", false, "stream CSV")
	fields := flags.String("fields", "", "comma-separated export fields")
	flags.DurationVar(&cfg.Interval, "interval", time.Second, "sample interval")
	flags.StringVar(&cfg.Output, "output", "-", "export destination")
	flags.DurationVar(&cfg.Duration, "duration", 0, "export duration")
	flags.IntVar(&cfg.Count, "count", 0, "export sample count")
	flags.BoolVar(&cfg.DCGM, "dcgm", false, "force DCGM activity")
	flags.BoolVar(&cfg.NoColor, "no-color", false, "disable color")
	flags.BoolVar(&cfg.Version, "version", false, "print version")

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if *jsonMode && *csvMode {
		return Config{}, ErrMutuallyExclusiveFormat
	}
	if cfg.Interval <= 0 {
		return Config{}, ErrInvalidInterval
	}
	if cfg.Count < 0 {
		return Config{}, ErrInvalidCount
	}
	if cfg.Duration < 0 {
		return Config{}, ErrInvalidDuration
	}

	switch {
	case *jsonMode:
		cfg.Mode = ModeJSON
	case *csvMode:
		cfg.Mode = ModeCSV
	}

	if strings.TrimSpace(*fields) != "" {
		for _, field := range strings.Split(*fields, ",") {
			if f := strings.TrimSpace(field); f != "" {
				cfg.Fields = append(cfg.Fields, f)
				continue
			}

			return Config{}, ErrInvalidFields
		}
	} else if *fields != "" {
		return Config{}, ErrInvalidFields
	}

	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("%w: %v", ErrUnexpectedArgs, flags.Args())
	}

	return cfg, nil
}
