//go:build dcgm

package dcgm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	nvidia "github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"nvcoretop/internal/gpu"
)

const fallbackNotice = "using NVML representational cores"

var activityFields = []nvidia.Short{
	nvidia.DCGM_FI_PROF_SM_ACTIVE,
	nvidia.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	nvidia.DCGM_FI_PROF_DRAM_ACTIVE,
	nvidia.DCGM_FI_PROF_PIPE_FP32_ACTIVE,
}

type dcgmAPI interface {
	initStandalone() (func(), error)
	initEmbedded() (func(), error)
	fieldGroupCreate(string, []nvidia.Short) (nvidia.FieldHandle, error)
	watchFields(uint, nvidia.FieldHandle, string) (nvidia.GroupHandle, error)
	updateAllFields() error
	getLatestValuesForFields(uint, []nvidia.Short) ([]nvidia.FieldValue_v1, error)
	destroyGroup(nvidia.GroupHandle) error
	fieldGroupDestroy(nvidia.FieldHandle) error
}

type realDCGM struct{}

func (realDCGM) initStandalone() (func(), error) {
	return nvidia.Init(nvidia.Standalone)
}

func (realDCGM) initEmbedded() (func(), error) {
	return nvidia.Init(nvidia.Embedded)
}

func (realDCGM) fieldGroupCreate(name string, fields []nvidia.Short) (nvidia.FieldHandle, error) {
	return nvidia.FieldGroupCreate(name, fields)
}

func (realDCGM) watchFields(gpuID uint, fieldsGroup nvidia.FieldHandle, groupName string) (nvidia.GroupHandle, error) {
	return nvidia.WatchFields(gpuID, fieldsGroup, groupName)
}

func (realDCGM) updateAllFields() error {
	return nvidia.UpdateAllFields()
}

func (realDCGM) getLatestValuesForFields(gpuID uint, fields []nvidia.Short) ([]nvidia.FieldValue_v1, error) {
	return nvidia.GetLatestValuesForFields(gpuID, fields)
}

func (realDCGM) destroyGroup(group nvidia.GroupHandle) error {
	return nvidia.DestroyGroup(group)
}

func (realDCGM) fieldGroupDestroy(fieldGroup nvidia.FieldHandle) error {
	return nvidia.FieldGroupDestroy(fieldGroup)
}

var dcgmClient dcgmAPI = realDCGM{}

type noop struct {
	notice string
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

type Client struct {
	mu         sync.Mutex
	api        dcgmAPI
	cleanup    func()
	fieldGroup nvidia.FieldHandle
	groups     []nvidia.GroupHandle
	mode       string
	active     bool
	closed     bool
	closeErr   error
}

func New(force bool, deviceCount int) (gpu.Enricher, error) {
	api := dcgmClient

	cleanup, mode, err := initDCGM(api)
	if err != nil {
		if force {
			return nil, fmt.Errorf("DCGM unavailable: %w", err)
		}
		return noop{notice: fmt.Sprintf("DCGM unavailable (%v); %s", err, fallbackNotice)}, nil
	}

	fieldGroup, err := api.fieldGroupCreate("nvcoretop-prof", activityFields)
	if err != nil {
		cleanup()
		if force {
			return nil, fmt.Errorf("DCGM fields unavailable: %w", err)
		}
		return noop{notice: fmt.Sprintf("DCGM fields unavailable (%v); %s", err, fallbackNotice)}, nil
	}

	client := &Client{
		api:        api,
		cleanup:    cleanup,
		fieldGroup: fieldGroup,
		mode:       mode,
		active:     true,
	}
	for i := 0; i < deviceCount; i++ {
		group, err := api.watchFields(uint(i), fieldGroup, fmt.Sprintf("nvcoretop-gpu-%d", i))
		if err != nil {
			closeErr := client.Close()
			watchErr := fmt.Errorf("DCGM watch unavailable for GPU %d: %w", i, err)
			if force {
				return nil, errors.Join(watchErr, closeErr)
			}
			return noop{notice: fmt.Sprintf("%v; %s", watchErr, fallbackNotice)}, nil
		}
		client.groups = append(client.groups, group)
	}
	return client, nil
}

func initDCGM(api dcgmAPI) (func(), string, error) {
	cleanup, standaloneErr := api.initStandalone()
	if standaloneErr == nil {
		return cleanup, "standalone", nil
	}

	cleanup, embeddedErr := api.initEmbedded()
	if embeddedErr == nil {
		return cleanup, "embedded", nil
	}

	return nil, "", fmt.Errorf("standalone init failed: %w; embedded init failed: %w", standaloneErr, embeddedErr)
}

func (c *Client) Enrich(ctx context.Context, snapshot gpu.Snapshot) (gpu.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return snapshot, gpu.ErrSamplerClosed
	}
	if err := c.api.updateAllFields(); err != nil {
		return snapshot, err
	}

	for i := range snapshot.Devices {
		values, err := c.api.getLatestValuesForFields(uint(snapshot.Devices[i].Index), activityFields)
		if err != nil {
			continue
		}
		for _, value := range values {
			applyActivityValue(&snapshot.Devices[i], value)
		}
	}
	snapshot.Source = gpu.SourceNVMLDCGM
	return snapshot, nil
}

func (c *Client) Active() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.active && !c.closed
}

func (c *Client) Notice() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return "DCGM active (" + c.mode + ")"
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return c.closeErr
	}
	c.closed = true
	c.active = false

	var errs []error
	for _, group := range c.groups {
		if group.GetHandle() != 0 {
			errs = append(errs, c.api.destroyGroup(group))
		}
	}
	c.groups = nil

	if c.fieldGroup.GetHandle() != 0 {
		errs = append(errs, c.api.fieldGroupDestroy(c.fieldGroup))
		c.fieldGroup = nvidia.FieldHandle{}
	}
	if c.cleanup != nil {
		c.cleanup()
		c.cleanup = nil
	}
	c.closeErr = errors.Join(errs...)
	return c.closeErr
}

func applyActivityValue(device *gpu.DeviceSample, value nvidia.FieldValue_v1) {
	if value.Status != nvidia.DCGM_ST_OK || value.FieldType != nvidia.DCGM_FT_DOUBLE {
		return
	}

	percent := normalizePercent(value.Float64())
	switch value.FieldID {
	case nvidia.DCGM_FI_PROF_SM_ACTIVE:
		device.SMActivePct = gpu.Some(percent)
	case nvidia.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
		device.TensorActivePct = gpu.Some(percent)
	case nvidia.DCGM_FI_PROF_DRAM_ACTIVE:
		device.MemPipeActivePct = gpu.Some(percent)
	case nvidia.DCGM_FI_PROF_PIPE_FP32_ACTIVE:
		device.FP32ActivePct = gpu.Some(percent)
	}
}

func normalizePercent(value float64) float64 {
	if value <= 1 {
		return value * 100
	}
	return value
}
