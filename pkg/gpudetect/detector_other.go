//go:build !linux

package gpudetect

import (
	"runtime"
)

type GPUType string

const (
	GPUTypeAppleSilicon     GPUType = "apple-silicon"
	GPUTypeNvidiaConsumer   GPUType = "nvidia-consumer"
	GPUTypeNvidiaDatacenter GPUType = "nvidia-datacenter"
	GPUTypeCPU              GPUType = "cpu"
)

func DetectGPU() (GPUType, error) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return GPUTypeAppleSilicon, nil
	}
	return GPUTypeCPU, nil
}