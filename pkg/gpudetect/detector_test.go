//go:build linux

package gpudetect

import (
	"testing"
)

func TestMapComputeCapability(t *testing.T) {
	tests := []struct {
		name     string
		major    int
		minor    int
		expected GPUType
	}{
		{"Turing (RTX 2080, GTX 1660 Ti, Tesla T4)", 7, 5, GPUTypeNvidiaConsumer},
		{"Ampere Consumer (RTX 3080, RTX A6000)", 8, 6, GPUTypeNvidiaConsumer},
		{"Ada Lovelace (RTX 4090)", 8, 9, GPUTypeNvidiaConsumer},
		{"Volta (V100)", 7, 0, GPUTypeNvidiaDatacenter},
		{"Ampere Datacenter (A100)", 8, 0, GPUTypeNvidiaDatacenter},
		{"Hopper (H100)", 9, 0, GPUTypeNvidiaDatacenter},
		{"Unknown architecture 6.1 (Pascal - older)", 6, 1, GPUTypeCPU},
		{"Unknown future architecture 10.0", 10, 0, GPUTypeCPU},
		{"Unknown minor version 7.3", 7, 3, GPUTypeCPU},
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

func TestParseComputeCapability(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantMajor   int
		wantMinor   int
		expectError bool
	}{
		{"valid single GPU", "8.9\n", 8, 9, false},
		{"valid no trailing newline", "7.5", 7, 5, false},
		{"multi-GPU uses first line", "8.9\n7.0\n", 8, 9, false},
		{"whitespace padding", "  8.6  \n", 8, 6, false},
		{"empty output", "", 0, 0, true},
		{"whitespace only", "   \n", 0, 0, true},
		{"no dot separator", "89\n", 0, 0, true},
		{"non-numeric major", "x.9\n", 0, 0, true},
		{"non-numeric minor", "8.x\n", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, err := parseComputeCapability(tt.output)
			if tt.expectError {
				if err == nil {
					t.Errorf("parseComputeCapability(%q) expected error, got %d.%d", tt.output, major, minor)
				}
				return
			}
			if err != nil {
				t.Errorf("parseComputeCapability(%q) unexpected error: %v", tt.output, err)
				return
			}
			if major != tt.wantMajor || minor != tt.wantMinor {
				t.Errorf("parseComputeCapability(%q) = %d.%d, want %d.%d", tt.output, major, minor, tt.wantMajor, tt.wantMinor)
			}
		})
	}
}
