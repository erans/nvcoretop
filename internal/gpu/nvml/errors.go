package nvml

import (
	"fmt"

	nvidia "github.com/NVIDIA/go-nvml/pkg/nvml"
)

func ok(ret nvidia.Return) bool {
	return ret == nvidia.SUCCESS
}

func errString(prefix string, ret nvidia.Return) error {
	return fmt.Errorf("%s: %s", prefix, nvidia.ErrorString(ret))
}
