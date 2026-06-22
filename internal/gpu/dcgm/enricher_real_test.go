//go:build dcgm

package dcgm

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	nvidia "github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"nvcoretop/internal/gpu"
)

func TestNewRealFallsBackToEmbeddedAndWatchesEachDevice(t *testing.T) {
	api := &fakeDCGMAPI{
		initStandaloneErr: errors.New("standalone unavailable"),
		fieldGroup:        fieldHandle(101),
		watchGroups: map[uint]nvidia.GroupHandle{
			0: groupHandle(201),
			1: groupHandle(202),
		},
	}
	restore := replaceDCGMClient(t, api)
	defer restore()

	enricher, err := New(false, 2)
	if err != nil {
		t.Fatalf("New(false, 2) error = %v", err)
	}
	if !enricher.Active() {
		t.Fatalf("Active() = false, want true")
	}
	if notice := enricher.Notice(); !strings.Contains(notice, "embedded") {
		t.Fatalf("Notice() = %q, want embedded mode", notice)
	}
	if !slices.Equal(api.initCalls, []string{"standalone", "embedded"}) {
		t.Fatalf("init calls = %#v, want standalone then embedded", api.initCalls)
	}
	if !slices.Equal(api.watchedGPUs, []uint{0, 1}) {
		t.Fatalf("watched GPUs = %#v, want [0 1]", api.watchedGPUs)
	}
	if !slices.Equal(api.fieldGroupFields, activityFields) {
		t.Fatalf("field group fields = %#v, want activity fields %#v", api.fieldGroupFields, activityFields)
	}

	if err := enricher.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := enricher.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if !slices.Equal(api.destroyedGroups, []uintptr{201, 202}) {
		t.Fatalf("destroyed groups = %#v, want [201 202]", api.destroyedGroups)
	}
	if !slices.Equal(api.destroyedFieldGroups, []uintptr{101}) {
		t.Fatalf("destroyed field groups = %#v, want [101]", api.destroyedFieldGroups)
	}
	if api.cleanupCalls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", api.cleanupCalls)
	}
}

func TestNewRealReturnsNoopOnSetupFailureUnlessForced(t *testing.T) {
	api := &fakeDCGMAPI{
		initStandaloneErr: errors.New("standalone unavailable"),
		initEmbeddedErr:   errors.New("embedded unavailable"),
	}
	restore := replaceDCGMClient(t, api)
	defer restore()

	enricher, err := New(false, 1)
	if err != nil {
		t.Fatalf("New(false, 1) error = %v", err)
	}
	if enricher.Active() {
		t.Fatalf("Active() = true, want inactive fallback")
	}
	if notice := enricher.Notice(); !strings.Contains(notice, "DCGM unavailable") || !strings.Contains(notice, "representational") {
		t.Fatalf("Notice() = %q, want useful fallback notice", notice)
	}

	_, err = New(true, 1)
	if err == nil || !strings.Contains(err.Error(), "DCGM unavailable") {
		t.Fatalf("New(true, 1) error = %v, want forced DCGM unavailable error", err)
	}
}

func TestNewRealCleansUpOnWatchFailure(t *testing.T) {
	api := &fakeDCGMAPI{
		fieldGroup: fieldHandle(301),
		watchGroups: map[uint]nvidia.GroupHandle{
			0: groupHandle(401),
		},
		watchErrs: map[uint]error{
			1: errors.New("watch failed"),
		},
	}
	restore := replaceDCGMClient(t, api)
	defer restore()

	enricher, err := New(false, 2)
	if err != nil {
		t.Fatalf("New(false, 2) error = %v", err)
	}
	if enricher.Active() {
		t.Fatalf("Active() = true, want inactive fallback")
	}
	if !slices.Equal(api.destroyedGroups, []uintptr{401}) {
		t.Fatalf("destroyed groups = %#v, want [401]", api.destroyedGroups)
	}
	if !slices.Equal(api.destroyedFieldGroups, []uintptr{301}) {
		t.Fatalf("destroyed field groups = %#v, want [301]", api.destroyedFieldGroups)
	}
	if api.cleanupCalls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", api.cleanupCalls)
	}

	_, err = New(true, 2)
	if err == nil || !strings.Contains(err.Error(), "DCGM watch unavailable") {
		t.Fatalf("New(true, 2) error = %v, want forced watch error", err)
	}
}

func TestRealEnrichMapsSuccessfulValuesAndSkipsFailedFields(t *testing.T) {
	api := &fakeDCGMAPI{
		latestValues: map[uint][]nvidia.FieldValue_v1{
			2: {
				floatField(nvidia.DCGM_FI_PROF_SM_ACTIVE, nvidia.DCGM_ST_OK, 0.25),
				floatField(nvidia.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE, nvidia.DCGM_ST_NOT_SUPPORTED, 0.50),
				floatField(nvidia.DCGM_FI_PROF_DRAM_ACTIVE, nvidia.DCGM_ST_OK, 70),
				floatField(nvidia.DCGM_FI_PROF_PIPE_FP32_ACTIVE, nvidia.DCGM_ST_OK, 0.5),
			},
		},
	}
	client := &Client{api: api, active: true}

	got, err := client.Enrich(context.Background(), gpu.Snapshot{
		Source:  gpu.SourceNVML,
		Devices: []gpu.DeviceSample{{Index: 2}},
	})
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}
	if api.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", api.updateCalls)
	}
	if got.Source != gpu.SourceNVMLDCGM {
		t.Fatalf("Source = %s, want NVML+DCGM", got.Source)
	}
	device := got.Devices[0]
	assertOptionalFloat(t, "SMActivePct", device.SMActivePct, 25)
	if device.TensorActivePct.OK {
		t.Fatalf("TensorActivePct = %#v, want missing for failed field", device.TensorActivePct)
	}
	assertOptionalFloat(t, "MemPipeActivePct", device.MemPipeActivePct, 70)
	assertOptionalFloat(t, "FP32ActivePct", device.FP32ActivePct, 50)
}

func TestRealEnrichRespectsContextCancellationBeforeDCGMWork(t *testing.T) {
	api := &fakeDCGMAPI{}
	client := &Client{api: api, active: true}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Enrich(ctx, gpu.Snapshot{Devices: []gpu.DeviceSample{{Index: 0}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Enrich() error = %v, want context canceled", err)
	}
	if api.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0 before canceled context", api.updateCalls)
	}
}

func assertOptionalFloat(t *testing.T, name string, got gpu.Optional[float64], want float64) {
	t.Helper()
	if !got.OK {
		t.Fatalf("%s missing, want %v", name, want)
	}
	if math.Abs(got.Value-want) > 0.0001 {
		t.Fatalf("%s = %v, want %v", name, got.Value, want)
	}
}

func replaceDCGMClient(t *testing.T, api dcgmAPI) func() {
	t.Helper()
	previous := dcgmClient
	dcgmClient = api
	return func() {
		dcgmClient = previous
	}
}

func fieldHandle(handle uintptr) nvidia.FieldHandle {
	var field nvidia.FieldHandle
	field.SetHandle(handle)
	return field
}

func groupHandle(handle uintptr) nvidia.GroupHandle {
	var group nvidia.GroupHandle
	group.SetHandle(handle)
	return group
}

func floatField(field nvidia.Short, status int, value float64) nvidia.FieldValue_v1 {
	var raw [4096]byte
	binary.LittleEndian.PutUint64(raw[:8], math.Float64bits(value))
	return nvidia.FieldValue_v1{
		FieldID:   field,
		FieldType: nvidia.DCGM_FT_DOUBLE,
		Status:    status,
		Value:     raw,
	}
}

type fakeDCGMAPI struct {
	initStandaloneErr error
	initEmbeddedErr   error
	initCalls         []string
	cleanupCalls      int

	fieldGroup       nvidia.FieldHandle
	fieldGroupErr    error
	fieldGroupFields []nvidia.Short

	watchGroups map[uint]nvidia.GroupHandle
	watchErrs   map[uint]error
	watchedGPUs []uint

	updateCalls int
	updateErr   error

	latestValues map[uint][]nvidia.FieldValue_v1
	latestErrs   map[uint]error

	destroyedGroups      []uintptr
	destroyGroupErr      error
	destroyedFieldGroups []uintptr
	destroyFieldGroupErr error
}

func (f *fakeDCGMAPI) initStandalone() (func(), error) {
	f.initCalls = append(f.initCalls, "standalone")
	if f.initStandaloneErr != nil {
		return nil, f.initStandaloneErr
	}
	return f.cleanup, nil
}

func (f *fakeDCGMAPI) initEmbedded() (func(), error) {
	f.initCalls = append(f.initCalls, "embedded")
	if f.initEmbeddedErr != nil {
		return nil, f.initEmbeddedErr
	}
	return f.cleanup, nil
}

func (f *fakeDCGMAPI) cleanup() {
	f.cleanupCalls++
}

func (f *fakeDCGMAPI) fieldGroupCreate(_ string, fields []nvidia.Short) (nvidia.FieldHandle, error) {
	f.fieldGroupFields = slices.Clone(fields)
	if f.fieldGroupErr != nil {
		return nvidia.FieldHandle{}, f.fieldGroupErr
	}
	if f.fieldGroup.GetHandle() == 0 {
		return fieldHandle(1), nil
	}
	return f.fieldGroup, nil
}

func (f *fakeDCGMAPI) watchFields(gpuID uint, _ nvidia.FieldHandle, _ string) (nvidia.GroupHandle, error) {
	f.watchedGPUs = append(f.watchedGPUs, gpuID)
	if err := f.watchErrs[gpuID]; err != nil {
		return nvidia.GroupHandle{}, err
	}
	if group, ok := f.watchGroups[gpuID]; ok {
		return group, nil
	}
	return groupHandle(uintptr(gpuID + 1)), nil
}

func (f *fakeDCGMAPI) updateAllFields() error {
	f.updateCalls++
	return f.updateErr
}

func (f *fakeDCGMAPI) getLatestValuesForFields(gpuID uint, _ []nvidia.Short) ([]nvidia.FieldValue_v1, error) {
	if err := f.latestErrs[gpuID]; err != nil {
		return nil, err
	}
	return slices.Clone(f.latestValues[gpuID]), nil
}

func (f *fakeDCGMAPI) destroyGroup(group nvidia.GroupHandle) error {
	f.destroyedGroups = append(f.destroyedGroups, group.GetHandle())
	return f.destroyGroupErr
}

func (f *fakeDCGMAPI) fieldGroupDestroy(fieldGroup nvidia.FieldHandle) error {
	f.destroyedFieldGroups = append(f.destroyedFieldGroups, fieldGroup.GetHandle())
	return f.destroyFieldGroupErr
}
