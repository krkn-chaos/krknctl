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
		Short: "AI-powered chaos engineering assistance",
		Long: `AI-powered chaos engineering assistance for krknctl

Available Commands:
  run      Run AI-powered chaos engineering assistance

The assist service uses a lightweight AI model with FAISS vector search
to provide intelligent command suggestions and documentation search.

Examples:
  krknctl assist run`,
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

This command deploys a lightweight AI model that can answer questions about krknctl usage,
chaos engineering scenarios, and provide intelligent command suggestions based on natural language.

The system uses:
- FAISS vector search for fast document retrieval
- Live documentation indexing
- Llama model optimized for chaos engineering domain

Examples:
  krknctl assist run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Print container runtime info
			(*scenarioOrchestrator).PrintContainerRuntime()

			// Check if Docker is being used - assist requires Podman
			if (*scenarioOrchestrator).GetContainerRuntime() == orchestratormodels.Docker {
				return fmt.Errorf("❌ assist requires Podman container runtime")
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

			// Deploy RAG model container
			fmt.Println("🚀 deploying assist model...")

			// Create spinners for the operations
			pullSpinner := NewSpinnerWithSuffix(" pulling RAG model image...")
			thinkingSpinner := NewSpinnerWithSuffix(" thinking...", 37)

			ragResult, err := assist.DeployAssistModel(ctx, *scenarioOrchestrator, config, registry, pullSpinner)
			if err != nil {
				return fmt.Errorf("failed to deploy RAG model: %w", err)
			}

			// Health check
			fmt.Println("\n🩺 performing health check...")

			healthOK, err := assist.PerformAssistHealthCheck(ragResult.ContainerID, ragResult.HostPort, *scenarioOrchestrator, ctx, config)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}

			if !healthOK {
				return fmt.Errorf("assist service is not responding properly")
			}

			fmt.Println("✅ assist service is ready!")

			// Start interactive prompt
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
