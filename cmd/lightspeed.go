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

GPU Types:
  Use one of these flags with any lightspeed command to specify your GPU type:
  
  --nvidia         NVIDIA GPUs (CUDA, GeForce, Quadro, Tesla)
  --amd           AMD GPUs (Radeon, FirePro, Instinct with ROCm)
  --intel         Intel GPUs (Arc, Iris, UHD Graphics)
  --apple-silicon Apple Silicon (M1, M2, M3, M4 with Metal)

Examples:
  krknctl lightspeed check --nvidia
  krknctl lightspeed check --apple-silicon`,
	}

	// Add GPU type flags
	command.PersistentFlags().Bool("nvidia", false, "Use NVIDIA GPU support")
	command.PersistentFlags().Bool("amd", false, "Use AMD GPU support")
	command.PersistentFlags().Bool("intel", false, "Use Intel GPU support")
	command.PersistentFlags().Bool("apple-silicon", false, "Use Apple Silicon GPU support")

	// Make GPU flags mutually exclusive and required
	command.MarkFlagsOneRequired("nvidia", "amd", "intel", "apple-silicon")
	command.MarkFlagsMutuallyExclusive("nvidia", "amd", "intel", "apple-silicon")

	return command
}

// getSelectedGPUType returns the selected GPU type from flags
func getSelectedGPUType(cmd *cobra.Command) string {
	if nvidia, _ := cmd.Flags().GetBool("nvidia"); nvidia {
		return "nvidia"
	}
	if amd, _ := cmd.Flags().GetBool("amd"); amd {
		return "amd"
	}
	if intel, _ := cmd.Flags().GetBool("intel"); intel {
		return "intel"
	}
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
  krknctl lightspeed check --amd
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