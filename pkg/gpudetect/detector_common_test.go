// Package gpudetect provides GPU detection functionality for selecting optimal container images.
// This test file works with both CGO and non-CGO builds.
package gpudetect

import (
	"testing"
)

// TestDetectGPU_PlatformDetection tests platform-based GPU detection
// Note: This test validates the platform detection logic and works with both CGO and non-CGO builds
func TestDetectGPU_PlatformDetection(t *testing.T) {
	// This test will pass on all platforms
	// It's a smoke test to ensure DetectGPU doesn't panic
	gpuType, err := DetectGPU()

	// Should always return a valid GPU type
	validTypes := []GPUType{
		GPUTypeAppleSilicon,
		GPUTypeNvidiaConsumer,
		GPUTypeNvidiaDatacenter,
		GPUTypeCPU,
	}

	found := false
	for _, valid := range validTypes {
		if gpuType == valid {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("DetectGPU() returned invalid GPU type: %v", gpuType)
	}

	// Error can be nil or non-nil depending on hardware and CGO availability
	// We just check it doesn't panic
	_ = err
}