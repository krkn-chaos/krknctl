//go:build !cgo

// Package gpudetect provides GPU detection functionality for selecting optimal container images.
// This file provides a stub implementation when CGO is disabled (e.g., during testing).
// It always returns GPUTypeCPU since NVML requires CGO.
package gpudetect

import (
	"runtime"
)

// GPUType represents the type of GPU detected on the system
type GPUType string

const (
	// GPUTypeAppleSilicon indicates Apple Silicon GPU (Metal via libkrun)
	GPUTypeAppleSilicon GPUType = "apple-silicon"
	// GPUTypeNvidiaConsumer indicates NVIDIA consumer GPU (RTX, GTX series)
	GPUTypeNvidiaConsumer GPUType = "nvidia-consumer"
	// GPUTypeNvidiaDatacenter indicates NVIDIA datacenter GPU (V100, A100, H100)
	GPUTypeNvidiaDatacenter GPUType = "nvidia-datacenter"
	// GPUTypeCPU indicates no GPU acceleration (CPU-only mode)
	GPUTypeCPU GPUType = "cpu"
)

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