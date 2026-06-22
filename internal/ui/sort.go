package ui

import (
	"sort"

	"nvcoretop/internal/gpu"
)

type SortMode int

const (
	SortIndex SortMode = iota
	SortUtil
	SortTemp
	SortMem
	SortPower
)

func (m SortMode) Next() SortMode {
	switch m {
	case SortIndex:
		return SortUtil
	case SortUtil:
		return SortTemp
	case SortTemp:
		return SortMem
	case SortMem:
		return SortPower
	default:
		return SortIndex
	}
}

func (m SortMode) String() string {
	switch m {
	case SortUtil:
		return "util"
	case SortTemp:
		return "temp"
	case SortMem:
		return "mem"
	case SortPower:
		return "power"
	default:
		return "index"
	}
}

func SortDevices(devices []gpu.DeviceSample, mode SortMode) []gpu.DeviceSample {
	out := make([]gpu.DeviceSample, len(devices))
	copy(out, devices)
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		switch mode {
		case SortUtil:
			return optionalUint32Desc(left.GPUUtil, right.GPUUtil, left.Index, right.Index)
		case SortTemp:
			return optionalUint32Desc(left.TempC, right.TempC, left.Index, right.Index)
		case SortMem:
			return ratioDesc(left.MemUsed, left.MemTotal, right.MemUsed, right.MemTotal, left.Index, right.Index)
		case SortPower:
			return optionalFloatDesc(left.PowerW, right.PowerW, left.Index, right.Index)
		default:
			return left.Index < right.Index
		}
	})
	return out
}

func optionalUint32Desc(left, right gpu.Optional[uint32], leftIndex, rightIndex int) bool {
	if left.OK != right.OK {
		return left.OK
	}
	if left.Value == right.Value {
		return leftIndex < rightIndex
	}
	return left.Value > right.Value
}

func optionalFloatDesc(left, right gpu.Optional[float64], leftIndex, rightIndex int) bool {
	if left.OK != right.OK {
		return left.OK
	}
	if left.Value == right.Value {
		return leftIndex < rightIndex
	}
	return left.Value > right.Value
}

func ratioDesc(leftUsed, leftTotal, rightUsed, rightTotal uint64, leftIndex, rightIndex int) bool {
	if leftTotal == 0 && rightTotal == 0 {
		return leftIndex < rightIndex
	}
	if leftTotal == 0 {
		return false
	}
	if rightTotal == 0 {
		return true
	}
	left := float64(leftUsed) / float64(leftTotal)
	right := float64(rightUsed) / float64(rightTotal)
	if left == right {
		return leftIndex < rightIndex
	}
	return left > right
}
