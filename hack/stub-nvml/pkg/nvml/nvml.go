// Package nvml provides a stub implementation of NVIDIA NVML library.
// This stub allows building without CGO when NVIDIA GPU features are not needed.
package nvml

// Stub types and functions to satisfy imports from krknctl
// These are never actually called in the operator context

type Return int
type Device struct{}

func (d Device) GetCudaComputeCapability() (int, int, Return) {
	return 0, 0, SUCCESS
}
type EccCounterType int

const (
	SUCCESS Return = 0
)

func Init() Return {
	return SUCCESS
}

func Shutdown() Return {
	return SUCCESS
}

func ErrorString(ret Return) string {
	return "stub nvml: no GPU support"
}

func DeviceGetCount() (int, Return) {
	return 0, SUCCESS
}

func DeviceGetHandleByIndex(index int) (Device, Return) {
	return Device{}, SUCCESS
}