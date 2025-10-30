// Package cmd contains the CLI commands for krknctl, including GPU and acceleration utilities.
// Assisted-by: Claude Sonnet 4
package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/assist"

	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	orchestratormodels "github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/spf13/cobra"
)

func NewAssistCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "assist",
		Short: "GPU and acceleration related utilities",
		Long: `GPU and acceleration related utilities for container runtime environments

Available Commands:
  check    Check GPU support in container runtime
  run      Run AI-powered chaos engineering assistance

GPU Auto-Detection:
  Lightspeed automatically detects your GPU type! No manual flags needed.
  
  Supported GPU Types:
  • Apple Silicon (M1, M2, M3, M4 with Metal)
  • NVIDIA GPUs (CUDA, GeForce, Quadro, Tesla)
  
  Future support planned:
  • AMD GPUs (Radeon, FirePro, Instinct with ROCm)
  • Intel GPUs (Arc, Iris, UHD Graphics)

Examples:
  krknctl assist check
  krknctl assist run
  krknctl assist run --no-gpu`,
	}

	// GPU auto-detection - no manual flags needed anymore!

	// Add no-gpu flag for CPU-only mode
	command.PersistentFlags().Bool("no-gpu", false, "Use CPU-only mode (no GPU acceleration)")

	return command
}

// GPU auto-detection - no manual flag parsing needed anymore!

func NewAssistCheckCommand(
	providerFactory *factory.ProviderFactory,
	scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator,
	config config.Config,
) *cobra.Command {
	var command = &cobra.Command{
		Use:   "check",
		Short: "Check GPU support in container runtime",
		Long: `Check whether the container runtime (Podman or Docker) has GPU support available

Lightspeed automatically tests all supported GPU types to detect your hardware.

Examples:
  krknctl assist check`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Print container runtime info
			(*scenarioOrchestrator).PrintContainerRuntime()

			// Check if Docker is being used - Lightspeed only supports Podman
			if (*scenarioOrchestrator).GetContainerRuntime() == orchestratormodels.Docker {
				return fmt.Errorf("❌ Assist requires Podman container runtime. " +
					"Docker is not supported for GPU acceleration")
			}

			// Get container runtime socket
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return fmt.Errorf("failed to get container runtime socket: %w", err)
			}

			// Connect to container runtime
			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return fmt.Errorf("failed to connect to container runtime: %w", err)
			}

			// Get no-gpu flag
			noGPU, _ := cmd.Flags().GetBool("no-gpu")

			// Create platform GPU detector
			detector := assist.NewPlatformGPUDetector(config)

			// Auto-detect GPU acceleration
			fmt.Println("\n🔍 Detecting GPU acceleration...")
			gpuType := detector.DetectGPUAcceleration(ctx, noGPU)

			// Get configuration
			imageURI, _, deviceMounts, err := detector.AutoSelectAssistConfig(ctx, noGPU)
			if err != nil {
				return fmt.Errorf("failed to get assist configuration: %w", err)
			}

			// Format and print result
			fmt.Printf("\n✅ GPU acceleration: %s\n", detector.GetGPUDescription(gpuType))
			fmt.Printf("📦 Container image: %s\n", imageURI)
			if len(deviceMounts) > 0 {
				fmt.Printf("🔗 Device mounts: %v\n", deviceMounts)
			} else {
				fmt.Printf("🔗 Device mounts: none (CPU-only)\n")
			}

			return nil
		},
	}

	return command
}

// buildAssistRegistryFromFlags builds assist registry configuration from command flags
func buildAssistRegistryFromFlags(cmd *cobra.Command, config config.Config) (*models.RegistryV2, error) {
	// Get private registry flags from parent command
	privateRegistry, _ := cmd.Flags().GetString("private-registry")
	privateRegistryAssist, _ := cmd.Flags().GetString("private-registry-assist")
	privateRegistryUsername, _ := cmd.Flags().GetString("private-registry-username")
	privateRegistryPassword, _ := cmd.Flags().GetString("private-registry-password")
	privateRegistryToken, _ := cmd.Flags().GetString("private-registry-token")
	privateRegistryInsecure, _ := cmd.Flags().GetBool("private-registry-insecure")
	privateRegistrySkipTLS, _ := cmd.Flags().GetBool("private-registry-skip-tls")

	// If no private registry is specified, return nil (use default public registry)
	if privateRegistry == "" {
		return nil, nil
	}

	// Use assist repository override if provided, otherwise use config default
	assistRepo := privateRegistryAssist
	if assistRepo == "" {
		assistRepo = config.AssistRegistry
	}

	// Build registry configuration
	registry := &models.RegistryV2{
		RegistryURL:        privateRegistry,
		Username:           &privateRegistryUsername,
		Password:           &privateRegistryPassword,
		Token:              &privateRegistryToken,
		Insecure:           privateRegistryInsecure,
		SkipTLS:            privateRegistrySkipTLS,
		ScenarioRepository: assistRepo,
	}

	return registry, nil
}

func NewAssistRunCommand(
	providerFactory *factory.ProviderFactory,
	scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator,
	config config.Config,
) *cobra.Command {
	var command = &cobra.Command{
		Use:   "run",
		Short: "Run AI-powered chaos engineering assistance",
		Long: `Run AI-powered chaos engineering assistance with Retrieval-Augmented Generation (RAG)

This command automatically detects your GPU and deploys a lightweight AI model that can answer 
questions about krknctl usage, chaos engineering scenarios, and provide intelligent command 
suggestions based on natural language.

The system uses:
- Automatic GPU detection (Apple Silicon, NVIDIA)
- GPU-accelerated inference for fast responses
- Live documentation indexing
- Llama 3.2:1B model optimized for chaos engineering domain

Examples:
  krknctl assist run          # Auto-detect GPU
  krknctl assist run --no-gpu # Force CPU-only mode`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flags
			noGPU, _ := cmd.Flags().GetBool("no-gpu")

			// Print container runtime info
			(*scenarioOrchestrator).PrintContainerRuntime()

			// Check if Docker is being used - Lightspeed only supports Podman
			if (*scenarioOrchestrator).GetContainerRuntime() == orchestratormodels.Docker {
				return fmt.Errorf("❌ assist requires Podman container runtime. " +
					"Docker is not supported for GPU acceleration")
			}

			// Get container runtime socket
			socket, err := (*scenarioOrchestrator).GetContainerRuntimeSocket(nil)
			if err != nil {
				return fmt.Errorf("failed to get container runtime socket: %w", err)
			}

			// Connect to container runtime
			ctx, err := (*scenarioOrchestrator).Connect(*socket)
			if err != nil {
				return fmt.Errorf("failed to connect to container runtime: %w", err)
			}

			// Build assist registry configuration from flags
			registry, err := buildAssistRegistryFromFlags(cmd, config)
			if err != nil {
				return fmt.Errorf("failed to build assist registry configuration: %w", err)
			}

			// Step 1: Auto-detect GPU acceleration
			fmt.Println("🔍 detecting GPU acceleration...")

			// Create platform GPU detector
			detector := assist.NewPlatformGPUDetector(config)
			gpuType := detector.DetectGPUAcceleration(ctx, noGPU)

			fmt.Printf("✅ GPU acceleration: %s\n", detector.GetGPUDescription(gpuType))

			// Step 2: Deploy RAG model container
			fmt.Println("\n🚀 deploying assist model...")

			// Create spinners for the operations
			pullSpinner := NewSpinnerWithSuffix(" pulling RAG model image...")
			thinkingSpinner := NewSpinnerWithSuffix(" thinking...", 37)

			ragResult, err := assist.DeployAssistModelWithGPUType(ctx, gpuType, *scenarioOrchestrator, config, registry, detector, pullSpinner)
			if err != nil {
				// Handle GPU-related errors with helpful suggestions
				enhancedErr := detector.HandleContainerError(err, gpuType)
				return fmt.Errorf("failed to deploy RAG model: %w", enhancedErr)
			}

			// Step 3: Health check
			fmt.Println("\n🩺 performing health check...")

			healthOK, err := assist.PerformAssistHealthCheck(ragResult.ContainerID, ragResult.HostPort, *scenarioOrchestrator, ctx, config)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			if !healthOK {
				return fmt.Errorf("assist service is not responding properly")
			}

			fmt.Println("✅ assist service is ready!")

			// Step 4: Start interactive prompt
			fmt.Printf("\n🚂 starting interactive assist service on port %s...\n",
				ragResult.HostPort)
			fmt.Println("type your chaos engineering questions and get intelligent krknctl" +
				" command suggestions!")
			fmt.Println("type 'exit' or 'quit' to stop.")

			// Create scenario provider for describe functionality
			scenarioProvider := providerFactory.NewInstance(provider.Quay)

			return assist.StartInteractivePrompt(ragResult.ContainerID, ragResult.HostPort, *scenarioOrchestrator, ctx, config, thinkingSpinner, scenarioProvider)
		},
	}

	return command
}
