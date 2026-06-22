//go:build !dcgm

package dcgm

import (
	"context"
	"errors"

	"nvcoretop/internal/gpu"
)

var ErrNotCompiled = errors.New("DCGM support not compiled; rebuild with -tags dcgm")

type noop struct {
	notice string
}

func New(force bool, deviceCount int) (gpu.Enricher, error) {
	if force {
		return nil, ErrNotCompiled
	}
	return noop{notice: "DCGM unavailable; using NVML representational cores"}, nil
}

func (n noop) Enrich(_ context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	return snapshot, nil
}

func (n noop) Active() bool {
	return false
}

func (n noop) Notice() string {
	return n.notice
}

func (n noop) Close() error {
	return nil
}
