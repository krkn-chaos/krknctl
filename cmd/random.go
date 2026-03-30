package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	providermodels "github.com/krkn-chaos/krknctl/pkg/provider/models"
	"github.com/krkn-chaos/krknctl/pkg/randomgraph"
	"github.com/krkn-chaos/krknctl/pkg/randomstate"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/models"
	"github.com/krkn-chaos/krknctl/pkg/scenarioorchestrator/utils"
	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/spf13/cobra"
)

func NewRandomCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "random",
		Short: "runs or scaffolds a random chaos run based on a json test plan",
		Long:  `runs or scaffolds a random chaos run based on a json test plan`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return command
}

// runRandomPlan contains the core execution logic shared by both attach and
// detach modes. It is called either directly (attach) or from a re-spawned
// child process (detach).
func runRandomPlan(
	cmd *cobra.Command,
	args []string,
	factory *providerfactory.ProviderFactory,
	scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator,
	cfg config.Config,
) error {
	registrySettings, err := providermodels.NewRegistryV2FromEnv(cfg)
	if err != nil {
		return err
	}
	if registrySettings == nil {
		registrySettings, err = parsePrivateRepoArgs(cmd, nil)
		if err != nil {
			return err
		}
	}

	(*scenarioOrchestrator).PrintContainerRuntime()
	if registrySettings != nil {
		logPrivateRegistry(registrySettings.RegistryURL)
	}

	// ── log directory ────────────────────────────────────────────────────────
	logDir, err := cmd.Flags().GetString("log-dir")
	if err != nil {
		return err
	}
	if logDir != "" {
		expanded, err := commonutils.ExpandFolder(logDir, nil)
		if err != nil {
			return err
		}
		logDir = *expanded
		if err = os.MkdirAll(logDir, 0750); err != nil {
			return fmt.Errorf("creating log directory %s: %w", logDir, err)
		}
	}

	// ── standard flags ───────────────────────────────────────────────────────
	volumes := make(map[string]string)
	environment := make(map[string]string)

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}
	if kubeconfig != "" {
		expandedConfig, err := commonutils.ExpandFolder(kubeconfig, nil)
		if err != nil {
			return err
		}
		kubeconfig = *expandedConfig
		if !CheckFileExists(kubeconfig) {
			return fmt.Errorf("file %s does not exist", kubeconfig)
		}
	}

	alertsProfile, err := cmd.Flags().GetString("alerts-profile")
	if err != nil {
		return err
	}
	if alertsProfile != "" {
		expandedProfile, err := commonutils.ExpandFolder(alertsProfile, nil)
		if err != nil {
			return err
		}
		alertsProfile = *expandedProfile
		if !CheckFileExists(alertsProfile) {
			return fmt.Errorf("file %s does not exist", alertsProfile)
		}
	}

	metricsProfile, err := cmd.Flags().GetString("metrics-profile")
	if err != nil {
		return err
	}
	if metricsProfile != "" {
		expandedProfile, err := commonutils.ExpandFolder(metricsProfile, nil)
		if err != nil {
			return err
		}
		metricsProfile = *expandedProfile
		if !CheckFileExists(metricsProfile) {
			return fmt.Errorf("file %s does not exist", metricsProfile)
		}
	}

	maxParallel, err := cmd.Flags().GetInt("max-parallel")
	if err != nil {
		return err
	}
	numberOfScenarios, err := cmd.Flags().GetInt("number-of-scenarios")
	if err != nil {
		return err
	}
	exitOnError, err := cmd.Flags().GetBool("exit-on-error")
	if err != nil {
		return err
	}
	randomGraphFile, err := cmd.Flags().GetString("graph-dump")
	if err != nil {
		return err
	}
	if randomGraphFile == "" {
		randomGraphFile = fmt.Sprintf(cfg.RandomGraphPath, time.Now().Unix())
	}

	kubeconfigPath, err := utils.PrepareKubeconfig(&kubeconfig, cfg)
	if err != nil {
		return err
	}
	if kubeconfigPath == nil {
		return fmt.Errorf("kubeconfig not found: %s", kubeconfig)
	}
	volumes[*kubeconfigPath] = cfg.KubeconfigPath

	if metricsProfile != "" {
		volumes[metricsProfile] = cfg.MetricsProfilePath
	}
	if alertsProfile != "" {
		volumes[alertsProfile] = cfg.AlertsProfilePath
	}

	// ── load plan ────────────────────────────────────────────────────────────
	file, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to open scenario file: %s", args[0])
	}
	nodes := make(map[string]models.ScenarioNode)
	if err = json.Unmarshal(file, &nodes); err != nil {
		return err
	}

	privateRegistry := registrySettings != nil
	dataProvider := GetProvider(privateRegistry, factory)

	// ── validate ─────────────────────────────────────────────────────────────
	spinner := NewSpinnerWithSuffix("running randomly generated chaos plan...")
	nameChannel := make(chan *struct {
		name *string
		err  error
	})
	spinner.Start()
	go func() {
		validateGraphScenarioInput(dataProvider, nodes, nameChannel, registrySettings)
	}()
	for {
		validateResult := <-nameChannel
		if validateResult == nil {
			break
		}
		if validateResult.err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to validate scenario: %s, error: %s", *validateResult.name, validateResult.err)
		}
		if validateResult.name != nil {
			spinner.Suffix = fmt.Sprintf("validating input for scenario: %s", *validateResult.name)
		}
	}
	spinner.Stop()

	// ── build execution plan ─────────────────────────────────────────────────
	executionPlan := randomgraph.NewRandomGraph(nodes, int64(maxParallel), numberOfScenarios)
	if err = DumpRandomGraph(nodes, executionPlan, randomGraphFile, cfg.LabelRootNode); err != nil {
		return err
	}
	if len(executionPlan) == 0 {
		_, err = color.New(color.FgYellow).Println("No scenario to execute; the random graph file appears to be empty (single-node graphs are not supported).")
		return err
	}

	table, err := NewGraphTable(executionPlan, cfg)
	if err != nil {
		return err
	}
	table.Print()
	fmt.Print("\n\n")

	// ── persist state ────────────────────────────────────────────────────────
	planName := args[0]
	state := &randomstate.State{
		PID:          os.Getpid(),
		ScenarioName: planName,
		PlanFile:     planName,
		StartTime:    time.Now(),
		LogDir:       logDir,
	}
	if err = randomstate.SaveState(state); err != nil {
		return fmt.Errorf("saving run state: %w", err)
	}
	defer func() { _ = randomstate.ClearState() }()

	// ── run ──────────────────────────────────────────────────────────────────
	spinner.Suffix = "starting chaos scenarios..."
	spinner.Start()

	commChannel := make(chan *models.GraphCommChannel)
	go func() {
		(*scenarioOrchestrator).RunGraph(nodes, executionPlan, environment, volumes, false, commChannel, registrySettings, nil)
	}()

	for {
		c := <-commChannel
		if c == nil {
			break
		}
		if c.Err != nil {
			spinner.Stop()
			var statErr *utils.ExitError
			if errors.As(c.Err, &statErr) {
				if c.ScenarioID != nil && c.ScenarioLogFile != nil {
					logFile := *c.ScenarioLogFile
					if logDir != "" {
						logFile = fmt.Sprintf("%s/%s", logDir, *c.ScenarioLogFile)
					}
					_, _ = color.New(color.FgHiRed).Println(fmt.Sprintf(
						"scenario %s at step %d with exit status %d, check log file %s aborting chaos run.",
						*c.ScenarioID, *c.Layer, statErr.ExitStatus, logFile))
				}
				if exitOnError {
					_, _ = color.New(color.FgHiRed).Println(fmt.Sprintf("aborting chaos run with exit status %d", statErr.ExitStatus))
					os.Exit(statErr.ExitStatus)
				}
				spinner.Start()
			}
		}
		if c != nil && c.Layer != nil {
			spinner.Suffix = fmt.Sprintf("Running step %d scenario(s): %s", *c.Layer, strings.Join(executionPlan[*c.Layer], ", "))
		}
	}
	spinner.Stop()
	return nil
}

func NewRandomRunCommand(factory *providerfactory.ProviderFactory, scenarioOrchestrator *scenarioorchestrator.ScenarioOrchestrator, cfg config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "run",
		Short: "runs a random chaos run",
		Long:  `runs a random run based on a json test plan`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			detach, err := cmd.Flags().GetBool("detach")
			if err != nil {
				return err
			}

			if detach {
				return runDetached(args)
			}
			return runRandomPlan(cmd, args, factory, scenarioOrchestrator, cfg)
		},
	}
	return command
}

// runDetached re-executes the current binary with --attach so the child runs
// the actual plan while the parent returns immediately.
func runDetached(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Rebuild the child argv: replace --detach with --attach
	childArgs := []string{"random", "run", "--attach"}
	for _, a := range os.Args[1:] {
		if a == "--detach" || a == "-d" {
			continue
		}
		childArgs = append(childArgs, a)
	}
	// Ensure the plan file is present
	if len(args) > 0 {
		// args[0] is already in os.Args, so it will be carried over above.
		// Nothing extra needed.
		_ = args
	}

	cmd := exec.Command(exe, childArgs...) // #nosec G204 -- exe is resolved from os.Executable(), not user input
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("starting background process: %w", err)
	}

	_, _ = color.New(color.FgGreen).Printf("🚀 chaos plan started in background (PID %d)\n", cmd.Process.Pid)
	return nil
}

// NewRandomStatusCommand returns the `random status` subcommand.
func NewRandomStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "shows the status of a running random chaos plan",
		Long:  `shows the status of a running random chaos plan`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := randomstate.LoadState()
			if err != nil {
				return fmt.Errorf("reading state: %w", err)
			}
			if state == nil {
				_, _ = color.New(color.FgYellow).Println("no random chaos plan is currently running")
				return nil
			}

			// Verify the process is still alive
			process, err := os.FindProcess(state.PID)
			running := false
			if err == nil {
				// On Unix, FindProcess always succeeds; send signal 0 to check
				if sigErr := process.Signal(os.Signal(nil)); sigErr == nil {
					running = true
				}
			}

			green := color.New(color.FgGreen).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			runningStr := red("false")
			if running {
				runningStr = green("true")
			}

			fmt.Printf("%s %s\n", bold("running:"), runningStr)
			fmt.Printf("%s %s\n", bold("scenario:"), state.ScenarioName)
			fmt.Printf("%s %d\n", bold("pid:"), state.PID)
			fmt.Printf("%s %s\n", bold("started:"), state.StartTime.Format(time.RFC3339))
			if state.LogDir != "" {
				fmt.Printf("%s %s\n", bold("log-dir:"), state.LogDir)
			}

			if !running {
				_, _ = color.New(color.FgYellow).Println("\nprocess is no longer alive; clearing stale state")
				_ = randomstate.ClearState()
			}
			return nil
		},
	}
}

// NewRandomAbortCommand returns the `random abort` subcommand.
func NewRandomAbortCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "abort",
		Short: "stops a running random chaos plan",
		Long:  `stops a running random chaos plan and cleans up its state`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			state, err := randomstate.LoadState()
			if err != nil {
				return fmt.Errorf("reading state: %w", err)
			}
			if state == nil {
				_, _ = color.New(color.FgYellow).Println("no random chaos plan is currently running")
				return nil
			}

			process, err := os.FindProcess(state.PID)
			if err != nil {
				_ = randomstate.ClearState()
				return fmt.Errorf("finding process %d: %w", state.PID, err)
			}

			if err = process.Kill(); err != nil {
				_ = randomstate.ClearState()
				return fmt.Errorf("killing process %d: %w", state.PID, err)
			}

			if err = randomstate.ClearState(); err != nil {
				return fmt.Errorf("clearing state: %w", err)
			}

			_, _ = color.New(color.FgGreen).Printf("✅ chaos plan (PID %d, scenario: %s) aborted successfully\n",
				state.PID, state.ScenarioName)
			return nil
		},
	}
}

func NewRandomScaffoldCommand(factory *providerfactory.ProviderFactory, cfg config.Config) *cobra.Command {
	var command = &cobra.Command{
		Use:   "scaffold",
		Short: "scaffolds a random chaos run",
		Long:  `scaffolds a random run based on a json test plan`,
		Args:  cobra.MinimumNArgs(0),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			registrySettings, err := providermodels.NewRegistryV2FromEnv(cfg)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
			}
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return nil, cobra.ShellCompDirectiveError
			}
			dataProvider := GetProvider(registrySettings != nil, factory)
			scenarios, err := FetchScenarios(dataProvider, registrySettings)
			if err != nil {
				log.Fatalf("Error fetching scenarios: %v", err)
				return []string{}, cobra.ShellCompDirectiveError
			}
			return *scenarios, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var seed *provider.ScaffoldSeed
			registrySettings, err := providermodels.NewRegistryV2FromEnv(cfg)
			if err != nil {
				return err
			}
			if registrySettings == nil {
				registrySettings, err = parsePrivateRepoArgs(cmd, nil)
				if err != nil {
					return err
				}
			}
			dataProvider := GetProvider(registrySettings != nil, factory)
			includeGlobalEnv, err := cmd.Flags().GetBool("global-env")
			if err != nil {
				return err
			}
			seedFile, err := cmd.Flags().GetString("seed-file")
			if err != nil {
				return err
			}
			numberOfScenarios, err := cmd.Flags().GetInt("number-of-scenarios")
			if err != nil {
				return err
			}
			if seedFile != "" {
				seedFilePath, err := commonutils.ExpandFolder(seedFile, nil)
				if err != nil {
					return err
				}
				if !CheckFileExists(*seedFilePath) {
					return fmt.Errorf("file %s does not exist", seedFile)
				}
				seed = &provider.ScaffoldSeed{
					NumberOfScenarios: numberOfScenarios,
					Path:              *seedFilePath,
				}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("please provide at least one scenario")
				}
			}
			output, err := dataProvider.ScaffoldScenarios(args, includeGlobalEnv, registrySettings, true, seed)
			if err != nil {
				return err
			}
			fmt.Println(*output)
			return nil
		},
	}
	return command
}
