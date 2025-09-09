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

GPU Types:
  Use one of these flags with any lightspeed command to specify your GPU type:
  
  --nvidia         NVIDIA GPUs (CUDA, GeForce, Quadro, Tesla)
  --apple-silicon Apple Silicon (M1, M2, M3, M4 with Metal)
  
  Temporarily disabled:
  --amd           AMD GPUs (Radeon, FirePro, Instinct with ROCm) [DISABLED]
  --intel         Intel GPUs (Arc, Iris, UHD Graphics) [DISABLED]

Examples:
  krknctl lightspeed check --nvidia
  krknctl lightspeed run --apple-silicon
  krknctl lightspeed run --nvidia --offline`,
	}

	// Add GPU type flags
	command.PersistentFlags().Bool("nvidia", false, "Use NVIDIA GPU support")
	// Temporarily disabled GPU types
	// command.PersistentFlags().Bool("amd", false, "Use AMD GPU support")
	// command.PersistentFlags().Bool("intel", false, "Use Intel GPU support")
	command.PersistentFlags().Bool("apple-silicon", false, "Use Apple Silicon GPU support")

	// Add offline flag for airgapped environments
	command.PersistentFlags().Bool("offline", false, "Use cached documentation (for airgapped environments)")

	// Make GPU flags mutually exclusive and required
	// Only require nvidia or apple-silicon for now
	command.MarkFlagsOneRequired("nvidia", "apple-silicon")
	command.MarkFlagsMutuallyExclusive("nvidia", "apple-silicon")

	return command
}

// getSelectedGPUType returns the selected GPU type from flags
func getSelectedGPUType(cmd *cobra.Command) string {
	if nvidia, _ := cmd.Flags().GetBool("nvidia"); nvidia {
		return "nvidia"
	}
	// Temporarily disabled GPU types
	// if amd, _ := cmd.Flags().GetBool("amd"); amd {
	//	return "amd"
	// }
	// if intel, _ := cmd.Flags().GetBool("intel"); intel {
	//	return "intel"
	// }
	if appleSilicon, _ := cmd.Flags().GetBool("apple-silicon"); appleSilicon {
		return "apple-silicon"
	}
	return ""
}

func NewLightspeedCheckCommand(
	providerFactory *factory.ProviderFactory,
	scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator,
	config config.Config,
) *cobra.Command {
	var command = &cobra.Command{
		Use:   "check",
		Short: "Check GPU support in container runtime",
		Long: `Check whether the container runtime (Podman or Docker) has GPU support available

Required: Exactly one GPU type flag must be specified.
Run 'krknctl lightspeed --help' to see available GPU types.

Examples:
  krknctl lightspeed check --nvidia
  krknctl lightspeed check --apple-silicon`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get selected GPU type
			gpuType := getSelectedGPUType(cmd)

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

			// Run GPU check with GPU-specific image
			result, err := gpuChecker.CheckGPUSupportByType(ctx, gpuType, registry)
			if err != nil {
				return fmt.Errorf("failed to check GPU support: %w", err)
			}

			// Format and print result
			fmt.Printf("GPU Type: %s\n\n", gpuType)
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

This command deploys a lightweight AI model that can answer questions about krknctl usage,
chaos engineering scenarios, and provide intelligent command suggestions based on natural language.

The system uses:
- GPU-accelerated inference for fast responses
- Live documentation indexing (or cached for offline environments)
- Llama 3.2:1B model optimized for chaos engineering domain

Required: Exactly one GPU type flag must be specified.
Run 'krknctl lightspeed --help' to see available GPU types.

Examples:
  krknctl lightspeed run --nvidia          # Use live documentation
  krknctl lightspeed run --apple-silicon --offline  # Use cached docs (airgapped)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get selected GPU type
			gpuType := getSelectedGPUType(cmd)

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

			// Step 1: Run GPU check first
			fmt.Printf("🔍 Checking GPU support for %s...\n", gpuType)
			
			// Build lightspeed registry configuration from flags
			registry, err := buildLightspeedRegistryFromFlags(cmd, config)
			if err != nil {
				return fmt.Errorf("failed to build lightspeed registry configuration: %w", err)
			}

			// Create GPU checker
			gpuChecker := gpucheck.NewGpuChecker(*scenarioOrchestrator, config)

			// Run GPU check with GPU-specific image
			result, err := gpuChecker.CheckGPUSupportByType(ctx, gpuType, registry)
			if err != nil {
				return fmt.Errorf("failed to check GPU support: %w", err)
			}

			// Check if GPU support is available
			if !result.HasGPUSupport {
				fmt.Printf("❌ GPU support check failed:\n\n")
				fmt.Println(gpuChecker.FormatResult(result))
				return fmt.Errorf("GPU support not available - cannot run lightspeed AI assistance")
			}

			fmt.Println("✅ GPU support confirmed!")

			// Step 2: Deploy RAG model container
			fmt.Println("\n🚀 Deploying AI assistance model...")
			
			ragResult, err := deployRAGModel(ctx, gpuType, offlineFlag, *scenarioOrchestrator, config, registry)
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
				return fmt.Errorf("RAG service is not responding properly")
			}

			fmt.Println("✅ AI assistance service is ready!")

			// Step 4: Start interactive prompt
			fmt.Printf("\n🤖 Starting interactive AI assistance on port %s...\n", ragResult.hostPort)
			fmt.Println("Type your chaos engineering questions and get intelligent krknctl command suggestions!")
			fmt.Println("Type 'exit' or 'quit' to stop.")

			return startInteractivePrompt(ragResult.containerID, ragResult.hostPort, *scenarioOrchestrator, ctx, config)
		},
	}

	return command
}