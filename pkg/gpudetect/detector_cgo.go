//go:build cgo

// Package gpudetect provides GPU detection functionality for selecting optimal container images.
// It uses NVML (NVIDIA Management Library) to detect NVIDIA GPU compute capabilities and
// maps them to consumer vs datacenter GPU types for optimized CUDA builds.
// This file is compiled only when CGO is enabled.
package gpudetect

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type GPUType string

const (
	GPUTypeAppleSilicon     GPUType = "apple-silicon"
	GPUTypeNvidiaConsumer   GPUType = "nvidia-consumer"
	GPUTypeNvidiaDatacenter GPUType = "nvidia-datacenter"
	GPUTypeCPU              GPUType = "cpu"
)

// DetectGPU returns the GPU type for container image selection.
// Returns GPUTypeCPU with warning if GPU detection is unavailable (graceful fallback).
func DetectGPU() (GPUType, error) {
	// macOS arm64: Apple Silicon GPU
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return GPUTypeAppleSilicon, nil
	}

	// Non-Linux platforms: CPU-only
	if runtime.GOOS != "linux" {
		return GPUTypeCPU, nil
	}

	// Linux: Check for NVIDIA device files (must be readable)
	// Use os.OpenRoot to scope file access under /dev (prevents directory traversal)
	devRoot, err := os.OpenRoot("/dev")
	if err != nil {
		// Cannot access /dev directory, use CPU mode
		return GPUTypeCPU, nil
	}
	defer func() {
		if err := devRoot.Close(); err != nil {
			log.Printf("Warning: failed to close /dev root: %v", err)
		}
	}()

	nvidiaDevices := []string{"nvidia0", "nvidiactl", "nvidia-uvm"}
	for _, dev := range nvidiaDevices {
		// Verify both existence and read access
		f, err := devRoot.Open(dev)
		if err != nil {
			// NVIDIA device not accessible (missing or permission denied), use CPU mode
			return GPUTypeCPU, nil
		}
		if err := f.Close(); err != nil {
			log.Printf("Warning: failed to close /dev/%s: %v", dev, err)
		}
	}

	// NVIDIA devices exist, try to detect GPU type
	gpuType, err := detectNvidiaGPUType()
	if err != nil {
		// Detection failed, fallback to CPU
		log.Printf("Warning: NVIDIA GPU detected but type detection failed: %v. Using CPU-only mode.", err)
		return GPUTypeCPU, err
	}

	return gpuType, nil
}

// detectNvidiaGPUType queries NVML for compute capability and maps to consumer/datacenter.
func detectNvidiaGPUType() (GPUType, error) {
	// Initialize NVML
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return GPUTypeCPU, fmt.Errorf("failed to initialize NVML: %s", nvml.ErrorString(ret))
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Printf("Warning: Failed to shutdown NVML: %s", nvml.ErrorString(ret))
		}
	}()

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return GPUTypeCPU, fmt.Errorf("failed to get device count: %s", nvml.ErrorString(ret))
	}

	if count == 0 {
		return GPUTypeCPU, fmt.Errorf("no NVIDIA devices found via NVML")
	}

	// Get first device
	device, ret := nvml.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		return GPUTypeCPU, fmt.Errorf("failed to get device handle: %s", nvml.ErrorString(ret))
	}

	// Query compute capability
	major, minor, ret := device.GetCudaComputeCapability()
	if ret != nvml.SUCCESS {
		return GPUTypeCPU, fmt.Errorf("failed to get compute capability: %s", nvml.ErrorString(ret))
	}

	// Map compute capability to GPU type
	gpuType := mapComputeCapability(major, minor)
	log.Printf("Detected NVIDIA GPU via NVML: Compute Capability %d.%d -> %s", major, minor, gpuType)

	return gpuType, nil
}

// mapComputeCapability maps CUDA compute capability to consumer/datacenter GPU type.
func mapComputeCapability(major, minor int) GPUType {
	switch {
	// Consumer GPUs
	case major == 7 && minor == 5: // Turing (RTX 20xx, GTX 1660 Ti, Tesla T4)
		return GPUTypeNvidiaConsumer
	case major == 8 && minor == 6: // Ampere Consumer (RTX 30xx, RTX A-series)
		return GPUTypeNvidiaConsumer
	case major == 8 && minor == 9: // Ada Lovelace (RTX 40xx)
		return GPUTypeNvidiaConsumer

	// Datacenter GPUs
	case major == 7 && minor == 0: // Volta (V100)
		return GPUTypeNvidiaDatacenter
	case major == 8 && minor == 0: // Ampere Datacenter (A100, A30)
		return GPUTypeNvidiaDatacenter
	case major == 9 && minor == 0: // Hopper (H100, H200)
		return GPUTypeNvidiaDatacenter

	// Unknown compute capability: fallback to CPU
	default:
		log.Printf("Warning: Unknown compute capability %d.%d, using CPU-only mode", major, minor)
		return GPUTypeCPU
	}
}
