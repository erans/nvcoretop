package history

type Ring struct {
	values []float64
	next   int
	full   bool
}

func NewRing(size int) *Ring {
	if size <= 0 {
		panic("history ring size must be positive")
	}
	return &Ring{values: make([]float64, size)}
}

func (r *Ring) Push(value float64) {
	r.values[r.next] = value
	r.next = (r.next + 1) % len(r.values)
	if r.next == 0 {
		r.full = true
	}
}

func (r *Ring) Values() []float64 {
	if !r.full {
		out := make([]float64, r.next)
		copy(out, r.values[:r.next])
		return out
	}

	out := make([]float64, len(r.values))
	copy(out, r.values[r.next:])
	copy(out[len(r.values)-r.next:], r.values[:r.next])
	return out
}
