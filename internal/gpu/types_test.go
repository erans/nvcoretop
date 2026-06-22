package gpu

import "testing"

func TestOptionalValues(t *testing.T) {
	missing := Optional[uint64]{}
	if missing.OK {
		t.Fatalf("zero optional should be missing")
	}

	present := Some(uint64(42))
	if !present.OK || present.Value != 42 {
		t.Fatalf("Some() = %#v, want present value 42", present)
	}
}

func TestThrottleReasonsActiveAndNames(t *testing.T) {
	tests := []struct {
		name       string
		reasons    ThrottleReasons
		wantActive bool
		wantNames  []string
	}{
		{
			name:       "zero-value",
			reasons:    ThrottleReasons{},
			wantActive: false,
			wantNames:  nil,
		},
		{
			name: "some flags",
			reasons: ThrottleReasons{
				Power:   true,
				Thermal: true,
			},
			wantActive: true,
			wantNames:  []string{"power", "thermal"},
		},
		{
			name: "all flags ordered",
			reasons: ThrottleReasons{
				GPUIdle:            true,
				ApplicationsClocks: true,
				SWPowerCap:         true,
				HWSlowdown:         true,
				SyncBoost:          true,
				SWThermal:          true,
				HWThermal:          true,
				HWPowerBrake:       true,
				Power:              true,
				Thermal:            true,
			},
			wantActive: true,
			wantNames:  []string{"idle", "app-clocks", "sw-power", "hw-slowdown", "sync-boost", "sw-thermal", "hw-thermal", "hw-power-brake", "power", "thermal"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.reasons.Active(); got != tt.wantActive {
				t.Fatalf("Active() = %v, want %v", got, tt.wantActive)
			}

			gotNames := tt.reasons.Names()
			if len(gotNames) != len(tt.wantNames) {
				t.Fatalf("Names() length = %d, want %d: %#v", len(gotNames), len(tt.wantNames), gotNames)
			}
			for i := range tt.wantNames {
				if gotNames[i] != tt.wantNames[i] {
					t.Fatalf("Names()[%d] = %q, want %q", i, gotNames[i], tt.wantNames[i])
				}
			}
		})
	}
}

func TestSourceString(t *testing.T) {
	tests := []struct {
		source Source
		want   string
	}{
		{source: SourceNVML, want: "NVML"},
		{source: SourceNVMLDCGM, want: "NVML+DCGM"},
		{source: Source(7), want: "Source(7)"},
	}
	for _, tt := range tests {
		if got := tt.source.String(); got != tt.want {
			t.Fatalf("%v.String() = %q, want %q", int(tt.source), got, tt.want)
		}
	}
}
