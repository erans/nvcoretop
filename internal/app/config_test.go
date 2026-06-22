package app

import (
	"errors"
	"testing"
	"time"
)

func TestParseArgsJSON(t *testing.T) {
	cfg, err := ParseArgs([]string{"--json", "--interval", "2s", "--count", "3", "--fields", "util,temp"})
	if err != nil {
		t.Fatalf("ParseArgs error = %v", err)
	}
	if cfg.Mode != ModeJSON || cfg.Interval != 2*time.Second || cfg.Count != 3 {
		t.Fatalf("config = %#v", cfg)
	}
	if len(cfg.Fields) != 2 || cfg.Fields[0] != "util" || cfg.Fields[1] != "temp" {
		t.Fatalf("fields = %#v", cfg.Fields)
	}
}

func TestParseArgsRejectsJSONAndCSV(t *testing.T) {
	_, err := ParseArgs([]string{"--json", "--csv"})
	if !errors.Is(err, ErrMutuallyExclusiveFormat) {
		t.Fatalf("ParseArgs error = %v, want ErrMutuallyExclusiveFormat", err)
	}
}

func TestParseArgsRejectsBadInterval(t *testing.T) {
	_, err := ParseArgs([]string{"--json", "--interval", "0s"})
	if !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("ParseArgs error = %v, want ErrInvalidInterval", err)
	}
}

func TestParseArgsDefaults(t *testing.T) {
	cfg, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs error = %v", err)
	}
	if cfg.Mode != ModeTUI || cfg.Interval != time.Second || cfg.Output != "-" || cfg.Count != 0 || cfg.Duration != 0 {
		t.Fatalf("defaults config = %#v", cfg)
	}
	if cfg.Fields != nil {
		t.Fatalf("expected nil fields, got %#v", cfg.Fields)
	}
}

func TestParseArgsCSV(t *testing.T) {
	cfg, err := ParseArgs([]string{"--csv"})
	if err != nil {
		t.Fatalf("ParseArgs error = %v", err)
	}
	if cfg.Mode != ModeCSV {
		t.Fatalf("config mode = %v, want ModeCSV", cfg.Mode)
	}
}

func TestParseArgsRejectsNegativeCount(t *testing.T) {
	_, err := ParseArgs([]string{"--count", "-1"})
	if !errors.Is(err, ErrInvalidCount) {
		t.Fatalf("ParseArgs error = %v, want ErrInvalidCount", err)
	}
}

func TestParseArgsRejectsNegativeDuration(t *testing.T) {
	_, err := ParseArgs([]string{"--duration", "-1s"})
	if !errors.Is(err, ErrInvalidDuration) {
		t.Fatalf("ParseArgs error = %v, want ErrInvalidDuration", err)
	}
}

func TestParseArgsRejectsUnexpectedArgs(t *testing.T) {
	_, err := ParseArgs([]string{"sample"})
	if err == nil || !errors.Is(err, ErrUnexpectedArgs) {
		t.Fatalf("ParseArgs error = %v, want wrapped ErrUnexpectedArgs", err)
	}
}

func TestParseArgsRejectsEmptyFieldsEntry(t *testing.T) {
	t.Run("double comma", func(t *testing.T) {
		_, err := ParseArgs([]string{"--fields", "util,,temp"})
		if !errors.Is(err, ErrInvalidFields) {
			t.Fatalf("ParseArgs error = %v, want ErrInvalidFields", err)
		}
	})
	t.Run("blank fields", func(t *testing.T) {
		_, err := ParseArgs([]string{"--fields", " , "})
		if !errors.Is(err, ErrInvalidFields) {
			t.Fatalf("ParseArgs error = %v, want ErrInvalidFields", err)
		}
	})
}

func TestParseArgsQuietErrorOutput(t *testing.T) {
	if _, err := ParseArgs([]string{"--does-not-exist"}); err == nil {
		t.Fatalf("ParseArgs expected error")
	}
}
