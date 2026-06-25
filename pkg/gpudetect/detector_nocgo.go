//go:build !cgo

// Package gpudetect provides GPU detection functionality for selecting optimal container images.
// This file provides a stub implementation when CGO is disabled (e.g., during testing).
// It always returns GPUTypeCPU since NVML requires CGO.
package gpudetect

import (
	"runtime"
)

// GPUType represents the type of GPU detected on the system
type GPUType int

const (
	// GPUTypeCPU indicates no GPU acceleration (CPU-only mode)
	GPUTypeCPU GPUType = iota
	// GPUTypeAppleSilicon indicates Apple Silicon GPU (Metal via libkrun)
	GPUTypeAppleSilicon
	// GPUTypeNvidiaConsumer indicates NVIDIA consumer GPU (RTX, GTX series)
	GPUTypeNvidiaConsumer
	// GPUTypeNvidiaDatacenter indicates NVIDIA datacenter GPU (V100, A100, H100)
	GPUTypeNvidiaDatacenter
)

// String returns a human-readable string representation of the GPU type
func (g GPUType) String() string {
	switch g {
	case GPUTypeCPU:
		return "CPU"
	case GPUTypeAppleSilicon:
		return "Apple Silicon"
	case GPUTypeNvidiaConsumer:
		return "NVIDIA Consumer"
	case GPUTypeNvidiaDatacenter:
		return "NVIDIA Datacenter"
	default:
		return "Unknown"
	}
}

// DetectGPU detects the GPU type available on the current system.
// This is a stub implementation that always returns GPUTypeCPU when CGO is disabled.
func DetectGPU() (GPUType, error) {
	// Without CGO, we can only detect Apple Silicon by platform
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return GPUTypeAppleSilicon, nil
	}

	// NVML requires CGO, so we return CPU for all other platforms
	return GPUTypeCPU, nil
}