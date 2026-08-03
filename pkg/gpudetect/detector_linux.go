//go:build linux

package gpudetect

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type GPUType string

const (
	GPUTypeAppleSilicon     GPUType = "apple-silicon"
	GPUTypeNvidiaConsumer   GPUType = "nvidia-consumer"
	GPUTypeNvidiaDatacenter GPUType = "nvidia-datacenter"
	GPUTypeCPU              GPUType = "cpu"
)

func DetectGPU() (GPUType, error) {
	devRoot, err := os.OpenRoot("/dev")
	if err != nil {
		return GPUTypeCPU, nil
	}
	defer func() {
		if err := devRoot.Close(); err != nil {
			log.Printf("Warning: failed to close /dev root: %v", err)
		}
	}()

	nvidiaDevices := []string{"nvidia0", "nvidiactl", "nvidia-uvm"}
	for _, dev := range nvidiaDevices {
		f, err := devRoot.Open(dev)
		if err != nil {
			return GPUTypeCPU, nil
		}
		if err := f.Close(); err != nil {
			log.Printf("Warning: failed to close /dev/%s: %v", dev, err)
		}
	}

	gpuType, err := detectNvidiaGPUType()
	if err != nil {
		log.Printf("Warning: NVIDIA GPU detected but type detection failed: %v. Using CPU-only mode.", err)
		return GPUTypeCPU, err
	}

	return gpuType, nil
}

func detectNvidiaGPUType() (GPUType, error) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return GPUTypeCPU, fmt.Errorf("nvidia-smi not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=compute_cap", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("nvidia-smi failed: %w", err)
	}

	major, minor, err := parseComputeCapability(string(out))
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to parse compute capability: %w", err)
	}

	gpuType := mapComputeCapability(major, minor)
	log.Printf("Detected NVIDIA GPU via nvidia-smi: Compute Capability %d.%d -> %s", major, minor, gpuType)

	return gpuType, nil
}

func parseComputeCapability(output string) (int, int, error) {
	line := strings.TrimSpace(strings.SplitN(output, "\n", 2)[0])
	if line == "" {
		return 0, 0, fmt.Errorf("empty nvidia-smi output")
	}

	parts := strings.SplitN(line, ".", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected format: %q", line)
	}

	major, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	minor, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	return major, minor, nil
}

func mapComputeCapability(major, minor int) GPUType {
	switch {
	case major == 7 && minor == 5:
		return GPUTypeNvidiaConsumer
	case major == 8 && minor == 6:
		return GPUTypeNvidiaConsumer
	case major == 8 && minor == 9:
		return GPUTypeNvidiaConsumer

	case major == 7 && minor == 0:
		return GPUTypeNvidiaDatacenter
	case major == 8 && minor == 0:
		return GPUTypeNvidiaDatacenter
	case major == 9 && minor == 0:
		return GPUTypeNvidiaDatacenter

	default:
		log.Printf("Warning: Unknown compute capability %d.%d, using CPU-only mode", major, minor)
		return GPUTypeCPU
	}
}
