package export

import (
	"errors"
	"testing"

	"nvcoretop/internal/gpu"
)

func TestResolveFieldsDefault(t *testing.T) {
	fields, err := ResolveFields(nil)
	if err != nil {
		t.Fatalf("ResolveFields(nil) error = %v", err)
	}
	want := []string{
		"i", "name", "uuid",
		"util", "mem_util", "mem_used", "mem_total", "temp",
		"power", "power_limit", "sm_clock", "mem_clock", "fan",
		"proc_count", "proc_mem",
		"pcie_tx", "pcie_rx", "nvlink_tx", "nvlink_rx",
		"ecc_sbe", "ecc_dbe",
		"sm_active", "tensor_active", "dram_active", "fp32_active",
	}
	if len(fields) != len(want) {
		t.Fatalf("default fields length = %d, want %d", len(fields), len(want))
	}
	for i := range want {
		if fields[i].Name != want[i] {
			t.Fatalf("field[%d] = %q, want %q", i, fields[i].Name, want[i])
		}
	}
}

func TestResolveFieldsDRAMActiveAndLegacyMemPipeAlias(t *testing.T) {
	fields, err := ResolveFields([]string{"dram_active", "mem_pipe_active"})
	if err != nil {
		t.Fatalf("ResolveFields DRAM fields error = %v", err)
	}
	sample := gpu.DeviceSample{MemPipeActivePct: gpu.Some(77.5)}

	for i, want := range []string{"dram_active", "mem_pipe_active"} {
		if fields[i].Name != want {
			t.Fatalf("field[%d] = %q, want %q", i, fields[i].Name, want)
		}
		value := fields[i].Value(sample)
		if value.JSON != 77.5 || value.CSV != "77.5" {
			t.Fatalf("%s value = %#v, want 77.5", want, value)
		}
	}
}

func TestResolveFieldsSubsetPreservesOrder(t *testing.T) {
	fields, err := ResolveFields([]string{"temp", "util", "power"})
	if err != nil {
		t.Fatalf("ResolveFields subset error = %v", err)
	}
	got := []string{fields[0].Name, fields[1].Name, fields[2].Name}
	want := []string{"temp", "util", "power"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveFieldsDuplicate(t *testing.T) {
	_, err := ResolveFields([]string{"temp", " temp ", "\ttemp"})
	if !errors.Is(err, ErrDuplicateField) {
		t.Fatalf("ResolveFields duplicate error = %v, want ErrDuplicateField", err)
	}
}

func TestResolveFieldsUnknown(t *testing.T) {
	_, err := ResolveFields([]string{"util", "bogus"})
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("ResolveFields unknown error = %v, want ErrUnknownField", err)
	}
}

func TestOptionalHelpersMissing(t *testing.T) {
	tests := []struct {
		name string
		got  FieldValue
	}{
		{"optionalUint32", optionalUint32(gpu.Optional[uint32]{})},
		{"optionalUint64", optionalUint64(gpu.Optional[uint64]{})},
		{"optionalFloat", optionalFloat(gpu.Optional[float64]{})},
	}

	for _, test := range tests {
		if test.got.JSON != nil {
			t.Fatalf("%s JSON = %#v, want nil", test.name, test.got.JSON)
		}
		if test.got.CSV != "" {
			t.Fatalf("%s CSV = %q, want \"\"", test.name, test.got.CSV)
		}
	}
}
