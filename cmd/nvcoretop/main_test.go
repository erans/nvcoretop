package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nvcoretop/internal/gpu"
	"nvcoretop/internal/ui"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if got, want := stdout.String(), "nvcoretop dev\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--json", "--count", "1", "--interval", "1ms"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'})
	if len(lines) != 1 {
		t.Fatalf("record count = %d, want 1; output = %q", len(lines), stdout.String())
	}

	var got map[string]any
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if got["source"] != "NVML" {
		t.Fatalf("source = %v, want NVML", got["source"])
	}
	gpus, ok := got["gpus"].([]any)
	if !ok {
		t.Fatalf("gpus type = %T, want []any", got["gpus"])
	}
	if len(gpus) != 0 {
		t.Fatalf("gpus = %#v, want empty", gpus)
	}
}

func TestRunDefaultModeDispatchesTUI(t *testing.T) {
	originalRunTUI := runTUI
	defer func() {
		runTUI = originalRunTUI
	}()

	var stdout, stderr bytes.Buffer
	called := false
	runTUI = func(ctx context.Context, sampler gpu.Sampler, interval time.Duration, options ui.Options) error {
		t.Helper()
		called = true

		if ctx == nil {
			t.Fatalf("ctx = nil, want context")
		}
		if interval != 250*time.Millisecond {
			t.Fatalf("interval = %s, want 250ms", interval)
		}
		if options.Interval != "250ms" {
			t.Fatalf("options.Interval = %q, want 250ms", options.Interval)
		}
		if !options.NoColor {
			t.Fatalf("options.NoColor = false, want true")
		}
		if !options.ForceDCGMView {
			t.Fatalf("options.ForceDCGMView = false, want true")
		}

		snapshot, err := sampler.Sample(ctx)
		if err != nil {
			t.Fatalf("sampler.Sample error = %v", err)
		}
		if snapshot.Source != gpu.SourceNVML {
			t.Fatalf("snapshot.Source = %s, want NVML", snapshot.Source)
		}
		return nil
	}

	if err := run([]string{"--interval", "250ms", "--no-color", "--dcgm"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if !called {
		t.Fatalf("runTUI was not called")
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	got := stdout.String()
	for _, want := range []string{"nvcoretop", "--json", "--csv"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output = %q, want to contain %q", got, want)
		}
	}
}

func TestRunInvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"--does-not-exist"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run error = nil, want error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
