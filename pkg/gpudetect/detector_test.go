package gpudetect

import (
	"testing"
)

// TestMapComputeCapability tests the mapping of CUDA compute capabilities to GPU types
func TestMapComputeCapability(t *testing.T) {
	tests := []struct {
		name     string
		major    int
		minor    int
		expected GPUType
	}{
		// Consumer GPUs
		{
			name:     "Turing (RTX 2080, GTX 1660 Ti, Tesla T4)",
			major:    7,
			minor:    5,
			expected: GPUTypeNvidiaConsumer,
		},
		{
			name:     "Ampere Consumer (RTX 3080, RTX A6000)",
			major:    8,
			minor:    6,
			expected: GPUTypeNvidiaConsumer,
		},
		{
			name:     "Ada Lovelace (RTX 4090)",
			major:    8,
			minor:    9,
			expected: GPUTypeNvidiaConsumer,
		},

		// Datacenter GPUs
		{
			name:     "Volta (V100)",
			major:    7,
			minor:    0,
			expected: GPUTypeNvidiaDatacenter,
		},
		{
			name:     "Ampere Datacenter (A100)",
			major:    8,
			minor:    0,
			expected: GPUTypeNvidiaDatacenter,
		},
		{
			name:     "Hopper (H100)",
			major:    9,
			minor:    0,
			expected: GPUTypeNvidiaDatacenter,
		},

		// Unknown architectures
		{
			name:     "Unknown architecture 6.1 (Pascal - older)",
			major:    6,
			minor:    1,
			expected: GPUTypeCPU,
		},
		{
			name:     "Unknown future architecture 10.0",
			major:    10,
			minor:    0,
			expected: GPUTypeCPU,
		},
		{
			name:     "Unknown minor version 7.3",
			major:    7,
			minor:    3,
			expected: GPUTypeCPU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapComputeCapability(tt.major, tt.minor)
			if result != tt.expected {
				t.Errorf("mapComputeCapability(%d, %d) = %v, want %v", tt.major, tt.minor, result, tt.expected)
			}
		})
	}
}

// TestDetectGPU_PlatformDetection tests platform-based GPU detection (Apple Silicon)
// Note: This test only validates the platform detection logic, not NVML functionality
func TestDetectGPU_PlatformDetection(t *testing.T) {
	// This test will only pass on actual macOS arm64 or Linux platforms
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

	// Error can be nil or non-nil depending on hardware
	// We just check it doesn't panic
	_ = err
}
