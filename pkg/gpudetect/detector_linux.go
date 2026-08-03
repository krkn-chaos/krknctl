//go:build linux

package gpudetect

import (
	"fmt"
	"log"
	"os"
	"unsafe"

	"github.com/ebitengine/purego"
)

type GPUType string

const (
	GPUTypeAppleSilicon     GPUType = "apple-silicon"
	GPUTypeNvidiaConsumer   GPUType = "nvidia-consumer"
	GPUTypeNvidiaDatacenter GPUType = "nvidia-datacenter"
	GPUTypeCPU              GPUType = "cpu"
)

const nvmlSuccess = 0

func dlsymVersioned(handle uintptr, name string) (uintptr, error) {
	sym, err := purego.Dlsym(handle, name+"_v2")
	if err == nil {
		return sym, nil
	}
	return purego.Dlsym(handle, name)
}

func goString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Add(unsafe.Pointer(ptr), n)) != 0 { // #nosec G103 -- reading null-terminated C string from NVML
		n++
	}
	return unsafe.String((*byte)(unsafe.Pointer(ptr)), n) // #nosec G103
}

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
	handle, err := purego.Dlopen("libnvidia-ml.so.1", purego.RTLD_LAZY)
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to load libnvidia-ml.so.1: %w", err)
	}

	fnInit, err := dlsymVersioned(handle, "nvmlInit")
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to find nvmlInit: %w", err)
	}

	fnShutdown, err := purego.Dlsym(handle, "nvmlShutdown")
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to find nvmlShutdown: %w", err)
	}

	fnErrStr, _ := purego.Dlsym(handle, "nvmlErrorString")

	fnGetCount, err := dlsymVersioned(handle, "nvmlDeviceGetCount")
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to find nvmlDeviceGetCount: %w", err)
	}

	fnGetHandle, err := dlsymVersioned(handle, "nvmlDeviceGetHandleByIndex")
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to find nvmlDeviceGetHandleByIndex: %w", err)
	}

	fnGetCC, err := purego.Dlsym(handle, "nvmlDeviceGetCudaComputeCapability")
	if err != nil {
		return GPUTypeCPU, fmt.Errorf("failed to find nvmlDeviceGetCudaComputeCapability: %w", err)
	}

	nvmlErrorString := func(ret uintptr) string {
		if fnErrStr != 0 {
			r, _, _ := purego.SyscallN(fnErrStr, ret)
			if s := goString(r); s != "" {
				return s
			}
		}
		return fmt.Sprintf("NVML error %d", ret)
	}

	ret, _, _ := purego.SyscallN(fnInit)
	if ret != nvmlSuccess {
		return GPUTypeCPU, fmt.Errorf("failed to initialize NVML: %s", nvmlErrorString(ret))
	}
	defer func() {
		r, _, _ := purego.SyscallN(fnShutdown)
		if r != nvmlSuccess {
			log.Printf("Warning: Failed to shutdown NVML: %s", nvmlErrorString(r))
		}
	}()

	var count uint32
	ret, _, _ = purego.SyscallN(fnGetCount, uintptr(unsafe.Pointer(&count))) // #nosec G103 -- passing Go pointer to NVML via purego FFI
	if ret != nvmlSuccess {
		return GPUTypeCPU, fmt.Errorf("failed to get device count: %s", nvmlErrorString(ret))
	}

	if count == 0 {
		return GPUTypeCPU, fmt.Errorf("no NVIDIA devices found via NVML")
	}

	var device uintptr
	ret, _, _ = purego.SyscallN(fnGetHandle, 0, uintptr(unsafe.Pointer(&device))) // #nosec G103
	if ret != nvmlSuccess {
		return GPUTypeCPU, fmt.Errorf("failed to get device handle: %s", nvmlErrorString(ret))
	}

	var major, minor int32
	ret, _, _ = purego.SyscallN(fnGetCC, device, uintptr(unsafe.Pointer(&major)), uintptr(unsafe.Pointer(&minor))) // #nosec G103
	if ret != nvmlSuccess {
		return GPUTypeCPU, fmt.Errorf("failed to get compute capability: %s", nvmlErrorString(ret))
	}

	gpuType := mapComputeCapability(int(major), int(minor))
	log.Printf("Detected NVIDIA GPU via NVML: Compute Capability %d.%d -> %s", major, minor, gpuType)

	return gpuType, nil
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
