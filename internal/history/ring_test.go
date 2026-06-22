package history

import "testing"

func TestRingPartialFill(t *testing.T) {
	r := NewRing(3)
	r.Push(10)
	r.Push(20)

	got := r.Values()
	want := []float64{10, 20}
	assertFloatSlice(t, got, want)
}

func TestRingWrapAround(t *testing.T) {
	r := NewRing(3)
	r.Push(10)
	r.Push(20)
	r.Push(30)
	r.Push(40)

	got := r.Values()
	want := []float64{20, 30, 40}
	assertFloatSlice(t, got, want)
}

func TestRingRejectsInvalidSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("NewRing(0) did not panic")
		}
	}()
	_ = NewRing(0)
}

func assertFloatSlice(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
