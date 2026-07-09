# GPU Support Stub for krknctl Consumers

## Overview

This directory contains a stub implementation of the NVIDIA NVML (NVIDIA Management Library) to enable krknctl consumers to build without CGO or NVIDIA GPU libraries.

## Background

krknctl uses NVIDIA NVML to detect GPU types (consumer vs datacenter) for optimized container image selection. However, NVML requires:
- **CGO enabled** during compilation
- **NVIDIA libraries** installed on the build system

This creates problems for projects that:
- Build with `CGO_ENABLED=0` (pure Go binaries)
- Run in environments without NVIDIA libraries (e.g., GitHub Actions runners, CI/CD pipelines)
- Don't need GPU detection functionality

## Solution

krknctl provides a **stub implementation** of NVML in `hack/stub-nvml/` that:
- ✅ Compiles without CGO
- ✅ Provides minimal API surface to satisfy imports
- ✅ Returns "no GPU detected" for all GPU detection calls
- ✅ Allows krknctl consumers to build successfully

## Default Behavior

**By default, krknctl uses real NVIDIA NVML** and includes full GPU detection support. This is the correct behavior for:
- krknctl release binaries
- Environments with NVIDIA GPUs
- Local development with GPU hardware

## For Consumers (krkn-operator, console, ACM, etc.)

If your project imports krknctl and encounters build errors like:

```
# github.com/NVIDIA/go-nvml/pkg/nvml
undefined: Return
undefined: EccCounterType
```

You need to use the stub. The recommended approach is to copy it locally:

### Local Copy (Recommended)

1. Copy `krknctl/hack/stub-nvml/` to your project's `hack/stub-nvml/`
2. Add replace directive pointing to local path in your `go.mod`:

```go
// Replace NVIDIA go-nvml with krknctl's stub to avoid CGO dependency
replace github.com/NVIDIA/go-nvml => ./hack/stub-nvml
```

3. Run:

```bash
go mod tidy
go mod vendor  # if using vendor
```

### Alternative: Direct Vendoring

If you're already vendoring dependencies, you can copy the stub directly into your `vendor/` directory at the appropriate path.

## What the Stub Provides

The stub implements the minimal NVML API used by krknctl's GPU detection:

```go
// Types
type Return int
type Device struct{}

// Constants
const SUCCESS Return = 0

// Functions
func Init() Return
func Shutdown() Return
func ErrorString(ret Return) string
func DeviceGetCount() (int, Return)
func DeviceGetHandleByIndex(index int) (Device, Return)

// Methods
func (d Device) GetCudaComputeCapability() (int, int, Return)
```

All functions return "success" states with zero/empty values, allowing code to compile and run with graceful "no GPU" behavior.

## Build Tags

krknctl uses build tags to select between real GPU detection and stub:

- **`detector_cgo.go`** (`//go:build cgo`): Real NVIDIA NVML detection
- **`detector_nocgo.go`** (`//go:build !cgo`): Stub that always returns CPU-only

When consumers use the replace directive, even with CGO enabled, the stub NVML implementation allows the no-CGO path to work correctly.

## Impact on Functionality

### With Real NVML (krknctl default)
- ✅ Detects NVIDIA GPUs
- ✅ Identifies consumer vs datacenter GPUs
- ✅ Selects optimized CUDA container images
- ✅ Falls back to CPU if no GPU detected

### With Stub (consumer using replace)
- ✅ Always returns "no GPU detected"
- ✅ Always selects CPU-only container images
- ✅ No CUDA optimization
- ⚠️ GPU detection functionality disabled

## Testing

To verify the stub works in your consumer project:

```bash
# With stub
CGO_ENABLED=0 go build -tags containers_image_openpgp ./...  # Should succeed

# Test that GPU detection returns CPU-only
CGO_ENABLED=0 go test -tags containers_image_openpgp -v ./... -run TestGPUDetection
```

## Maintenance

The stub API surface is minimal and stable. It only needs updates if:
- krknctl adds new NVML function calls
- NVML API changes in ways that affect krknctl's usage

When updating, ensure the stub provides all functions/types that `pkg/gpudetect/detector_cgo.go` imports.

## Questions?

For issues with GPU detection or the stub implementation, please file an issue at:
https://github.com/krkn-chaos/krknctl/issues