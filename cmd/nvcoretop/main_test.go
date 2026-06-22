package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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

func TestRunDefaultMode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run(nil, &stdout, &stderr); err != nil {
		if !errors.Is(err, ErrTUIUnavailable) {
			t.Fatalf("run error = %v, want ErrTUIUnavailable", err)
		}
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "interactive TUI mode will be enabled by the UI plan") {
		t.Fatalf("stderr = %q, want TUI message", got)
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
