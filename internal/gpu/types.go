package gpu

import (
	"context"
	"fmt"
	"time"
)

type Optional[T any] struct {
	Value T
	OK    bool
}

func Some[T any](value T) Optional[T] {
	return Optional[T]{Value: value, OK: true}
}

type Source int

const (
	SourceNVML Source = iota
	SourceNVMLDCGM
)

func (s Source) String() string {
	switch s {
	case SourceNVML:
		return "NVML"
	case SourceNVMLDCGM:
		return "NVML+DCGM"
	default:
		return fmt.Sprintf("Source(%d)", s)
	}
}

type Snapshot struct {
	Timestamp time.Time
	Source    Source
	Devices   []DeviceSample
}

type DeviceSample struct {
	Index int
	Name  string
	UUID  string

	MemUsed  uint64
	MemTotal uint64
	GPUUtil  Optional[uint32]
	MemUtil  Optional[uint32]
	TempC    Optional[uint32]

	PowerW      Optional[float64]
	PowerLimitW Optional[float64]

	SMClockMHz      Optional[uint32]
	MemClockMHz     Optional[uint32]
	ThrottleReasons ThrottleReasons
	FanPct          Optional[uint32]

	Processes []Process

	PCIeTxKBps       Optional[uint64]
	PCIeRxKBps       Optional[uint64]
	NVLinkTxKBps     Optional[uint64]
	NVLinkRxKBps     Optional[uint64]
	ECCSingleBit     Optional[uint64]
	ECCDoubleBit     Optional[uint64]
	SMActivePct      Optional[float64]
	TensorActivePct  Optional[float64]
	MemPipeActivePct Optional[float64]
	FP32ActivePct    Optional[float64]

	ProcessLimited bool
}

type Process struct {
	PID     uint32
	Name    string
	MemUsed uint64
}

type ThrottleReasons struct {
	GPUIdle            bool
	ApplicationsClocks bool
	SWPowerCap         bool
	HWSlowdown         bool
	SyncBoost          bool
	SWThermal          bool
	HWThermal          bool
	HWPowerBrake       bool
	Power              bool
	Thermal            bool
}

func (r ThrottleReasons) Active() bool {
	return len(r.Names()) > 0
}

func (r ThrottleReasons) Names() []string {
	names := make([]string, 0, 10)
	if r.GPUIdle {
		names = append(names, "idle")
	}
	if r.ApplicationsClocks {
		names = append(names, "app-clocks")
	}
	if r.SWPowerCap {
		names = append(names, "sw-power")
	}
	if r.HWSlowdown {
		names = append(names, "hw-slowdown")
	}
	if r.SyncBoost {
		names = append(names, "sync-boost")
	}
	if r.SWThermal {
		names = append(names, "sw-thermal")
	}
	if r.HWThermal {
		names = append(names, "hw-thermal")
	}
	if r.HWPowerBrake {
		names = append(names, "hw-power-brake")
	}
	if r.Power {
		names = append(names, "power")
	}
	if r.Thermal {
		names = append(names, "thermal")
	}
	return names
}

type Sampler interface {
	Sample(context.Context) (Snapshot, error)
	DeviceCount() int
	Close() error
}
