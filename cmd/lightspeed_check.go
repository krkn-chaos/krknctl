// Package cmd contains the CLI commands for krknctl, including GPU and acceleration utilities.
// Assisted-by: Claude Sonnet 4
package cmd

import (
	"fmt"

	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/gpucheck"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/spf13/cobra"
)

func NewLightspeedCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "lightspeed",
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
  krknctl lightspeed check
  krknctl lightspeed run
  krknctl lightspeed run --offline`,
	}

	// GPU auto-detection - no manual flags needed anymore!

	// Add offline flag for airgapped environments
	command.PersistentFlags().Bool("offline", false, "Use cached documentation (for airgapped environments)")

	return command
}

// GPU auto-detection - no manual flag parsing needed anymore!

func NewLightspeedCheckCommand(
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
  krknctl lightspeed check`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Print container runtime info
			(*scenarioOrchestrator).PrintContainerRuntime()

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

			// Build lightspeed registry configuration from flags
			registry, err := buildLightspeedRegistryFromFlags(cmd, config)
			if err != nil {
				return fmt.Errorf("failed to build lightspeed registry configuration: %w", err)
			}

			// Create GPU checker
			gpuChecker := gpucheck.NewGpuChecker(*scenarioOrchestrator, config)

			// Auto-detect GPU support
			fmt.Println("\n🔍 Auto-detecting GPU support...")
			result, err := gpuChecker.AutoDetectGPU(ctx, registry)
			if err != nil {
				return fmt.Errorf("failed to auto-detect GPU support: %w", err)
			}

			// Format and print result
			if result.HasGPUSupport {
				fmt.Printf("\n✅ GPU support confirmed: %s\n", result.GPUType)
			} else {
				fmt.Printf("\n❌ No GPU support detected\n")
			}
			fmt.Println(gpuChecker.FormatResult(result))

			return nil
		},
	}

	return command
}

// buildLightspeedRegistryFromFlags builds lightspeed registry configuration from command flags
func buildLightspeedRegistryFromFlags(cmd *cobra.Command, config config.Config) (*models.RegistryV2, error) {
	// Get private registry flags from parent command
	privateRegistry, _ := cmd.Flags().GetString("private-registry")
	privateRegistryLightspeed, _ := cmd.Flags().GetString("private-registry-lightspeed")
	privateRegistryUsername, _ := cmd.Flags().GetString("private-registry-username")
	privateRegistryPassword, _ := cmd.Flags().GetString("private-registry-password")
	privateRegistryToken, _ := cmd.Flags().GetString("private-registry-token")
	privateRegistryInsecure, _ := cmd.Flags().GetBool("private-registry-insecure")
	privateRegistrySkipTLS, _ := cmd.Flags().GetBool("private-registry-skip-tls")

	// If no private registry is specified, return nil (use default public registry)
	if privateRegistry == "" {
		return nil, nil
	}

	// Use lightspeed repository override if provided, otherwise use config default
	lightspeedRepo := privateRegistryLightspeed
	if lightspeedRepo == "" {
		lightspeedRepo = config.LightspeedRegistry
	}

	// Build registry configuration
	registry := &models.RegistryV2{
		RegistryURL:        privateRegistry,
		Username:           &privateRegistryUsername,
		Password:           &privateRegistryPassword,
		Token:              &privateRegistryToken,
		Insecure:           privateRegistryInsecure,
		SkipTLS:            privateRegistrySkipTLS,
		ScenarioRepository: lightspeedRepo,
	}

	return registry, nil
}

func NewLightspeedRunCommand(
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
- Live documentation indexing (or cached for offline environments)
- Llama 3.2:1B model optimized for chaos engineering domain

Examples:
  krknctl lightspeed run          # Auto-detect GPU, use live documentation
  krknctl lightspeed run --offline  # Auto-detect GPU, use cached docs (airgapped)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get offline flag
			offlineFlag, _ := cmd.Flags().GetBool("offline")

			// Print container runtime info
			(*scenarioOrchestrator).PrintContainerRuntime()

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

			// Build lightspeed registry configuration from flags
			registry, err := buildLightspeedRegistryFromFlags(cmd, config)
			if err != nil {
				return fmt.Errorf("failed to build lightspeed registry configuration: %w", err)
			}

			// Step 1: Auto-detect GPU support
			fmt.Println("🔍 Auto-detecting GPU support...")
			
			// Create GPU checker
			gpuChecker := gpucheck.NewGpuChecker(*scenarioOrchestrator, config)

			// Auto-detect GPU support
			result, err := gpuChecker.AutoDetectGPU(ctx, registry)
			if err != nil {
				return fmt.Errorf("failed to auto-detect GPU support: %w", err)
			}

			// Check if GPU support is available
			if !result.HasGPUSupport {
				fmt.Printf("❌ GPU auto-detection failed:\n\n")
				fmt.Println(gpuChecker.FormatResult(result))
				return fmt.Errorf("no GPU support found - cannot run Lightspeed service")
			}

			fmt.Printf("✅ GPU support confirmed: %s\n", result.GPUType)

			// Step 2: Deploy RAG model container
			fmt.Println("\n🚀 Deploying lightspeed model...")
			
			ragResult, err := deployRAGModel(ctx, result.GPUType, offlineFlag, *scenarioOrchestrator, config, registry)
			if err != nil {
				return fmt.Errorf("failed to deploy RAG model: %w", err)
			}

			// Step 3: Health check
			fmt.Println("\n🩺 Performing health check...")
			
			healthOK, err := performRAGHealthCheck(ragResult.containerID, ragResult.hostPort, *scenarioOrchestrator, ctx, config)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}
			
			if !healthOK {
				return fmt.Errorf("lightspeed service is not responding properly")
			}

			fmt.Println("✅ Lightspeed service is ready!")

			// Step 4: Start interactive prompt
			fmt.Printf("\n🤖 Starting interactive Lightspeed service on port %s...\n", ragResult.hostPort)
			fmt.Println("Type your chaos engineering questions and get intelligent krknctl command suggestions!")
			fmt.Println("Type 'exit' or 'quit' to stop.")

			return startInteractivePrompt(ragResult.containerID, ragResult.hostPort, *scenarioOrchestrator, ctx, config)
		},
	}

	return command
}