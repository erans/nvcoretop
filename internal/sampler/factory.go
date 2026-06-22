package sampler

import (
	"errors"
	"time"

	"nvcoretop/internal/gpu"
	"nvcoretop/internal/gpu/dcgm"
	"nvcoretop/internal/gpu/nvml"
)

type Options struct {
	ForceDCGM bool
	Now       func() time.Time
}

type Result struct {
	Sampler gpu.Sampler
	Notice  string
}

var (
	newNVML = func(options nvml.Options) (gpu.Sampler, error) {
		return nvml.New(options)
	}
	newDCGM = dcgm.New
)

func New(options Options) (Result, error) {
	base, err := newNVML(nvml.Options{Now: options.Now})
	if err != nil {
		return Result{}, err
	}

	enricher, dcgmErr := newDCGM(options.ForceDCGM, base.DeviceCount())
	if dcgmErr != nil {
		return Result{}, errors.Join(dcgmErr, base.Close())
	}
	if closeErr := base.Close(); closeErr != nil {
		return Result{}, errors.Join(closeErr, enricher.Close())
	}

	created, err := newNVML(nvml.Options{
		Now:      options.Now,
		Enricher: enricher,
	})
	if err != nil {
		return Result{}, errors.Join(err, enricher.Close())
	}
	return Result{Sampler: created, Notice: enricher.Notice()}, nil
}
