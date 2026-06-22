package history

import "nvcoretop/internal/gpu"

type DeviceHistory struct {
	Util  *Ring
	Temp  *Ring
	Power *Ring
}

type Store struct {
	window  int
	devices map[int]*DeviceHistory
}

func NewStore(window int) *Store {
	if window <= 0 {
		panic("history window must be positive")
	}
	return &Store{
		window:  window,
		devices: make(map[int]*DeviceHistory),
	}
}

func (s *Store) Add(snapshot gpu.Snapshot) {
	for _, device := range snapshot.Devices {
		history := s.ensure(device.Index)
		if device.GPUUtil.OK {
			history.Util.Push(float64(device.GPUUtil.Value))
		}
		if device.TempC.OK {
			history.Temp.Push(float64(device.TempC.Value))
		}
		if device.PowerW.OK {
			history.Power.Push(device.PowerW.Value)
		}
	}
}

func (s *Store) Device(index int) (DeviceHistory, bool) {
	history, ok := s.devices[index]
	if !ok {
		return DeviceHistory{}, false
	}
	return cloneDeviceHistory(history), true
}

func (s *Store) ensure(index int) *DeviceHistory {
	if history, ok := s.devices[index]; ok {
		return history
	}
	history := &DeviceHistory{
		Util:  NewRing(s.window),
		Temp:  NewRing(s.window),
		Power: NewRing(s.window),
	}
	s.devices[index] = history
	return history
}

func cloneDeviceHistory(history *DeviceHistory) DeviceHistory {
	return DeviceHistory{
		Util:  cloneRing(history.Util),
		Temp:  cloneRing(history.Temp),
		Power: cloneRing(history.Power),
	}
}

func cloneRing(r *Ring) *Ring {
	cloned := &Ring{
		values: make([]float64, len(r.values)),
		next:   r.next,
		full:   r.full,
	}
	copy(cloned.values, r.values)
	return cloned
}
