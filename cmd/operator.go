package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/krkn-chaos/krknctl/pkg/config"
	commonutils "github.com/krkn-chaos/krknctl/pkg/utils"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

func NewOperatorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "operator",
		Short: "manage the krkn operator",
		Long:  "Commands for installing and managing the krkn operator on Kubernetes clusters",
	}
}

func NewOperatorInstallCommand(cfg config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "install the krkn operator on a cluster",
		Long: `Installs the krkn operator using the Helm SDK.

Provide exactly one of:
  --kind        spin up a new single-node KinD cluster then deploy
  --kubeconfig  deploy to an existing cluster via the given kubeconfig`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			useKind, _ := cmd.Flags().GetBool("kind")
			kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			namespace, _ := cmd.Flags().GetString("namespace")
			chartPath, _ := cmd.Flags().GetString("chart-path")
			operatorVersion, _ := cmd.Flags().GetString("operator-version")
			portFwd, _ := cmd.Flags().GetBool("port-forward")
			localPort, _ := cmd.Flags().GetInt("local-port")

			if !useKind && kubeconfig == "" {
				return fmt.Errorf("one of --kind or --kubeconfig must be specified")
			}

			var kubeconfigPath string

			if useKind {
				var err error
				kubeconfigPath, err = kindCreateCluster(clusterName)
				if err != nil {
					return err
				}
			} else {
				expanded, err := commonutils.ExpandFolder(kubeconfig, nil)
				if err != nil {
					return fmt.Errorf("invalid kubeconfig path: %w", err)
				}
				kubeconfigPath = *expanded
			}

			usingLocalChart := chartPath != ""
			chartRef := cfg.OperatorChartURL
			if usingLocalChart {
				chartRef = chartPath
			}

			version := ""
			if !usingLocalChart {
				version = operatorVersion
			}

			fmt.Printf("Installing krkn-operator into namespace %q using chart %q\n", namespace, chartRef)
			if err := helmInstall(kubeconfigPath, namespace, chartRef, "krkn-operator", version); err != nil {
				return fmt.Errorf("helm install failed: %w", err)
			}

			fmt.Printf("krkn-operator installed successfully in namespace %q\n", namespace)
			if useKind {
				fmt.Printf("KinD cluster %q is running — retrieve its kubeconfig with: kind get kubeconfig --name %s\n", clusterName, clusterName)
			}

			if !portFwd {
				return nil
			}

			remotePort := cfg.OperatorConsoleRemotePort
			fmt.Printf("Starting background port-forward: http://localhost:%d → %s/krkn-operator-console:%d\n", localPort, namespace, remotePort)
			return portForwardConsole(kubeconfigPath, namespace, "krkn-operator-console", localPort, remotePort)
		},
	}
}

func NewOperatorUninstallCommand(cfg config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "uninstall the krkn operator from a cluster",
		Long: `Removes the krkn operator using the Helm SDK.

Provide exactly one of:
  --kind        delete the entire KinD cluster (removes everything)
  --kubeconfig  helm-uninstall the operator from an existing cluster`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			useKind, _ := cmd.Flags().GetBool("kind")
			kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
			clusterName, _ := cmd.Flags().GetString("cluster-name")
			namespace, _ := cmd.Flags().GetString("namespace")
			deleteNamespace, _ := cmd.Flags().GetBool("delete-namespace")

			if !useKind && kubeconfig == "" {
				return fmt.Errorf("one of --kind or --kubeconfig must be specified")
			}

			if useKind {
				stopBackgroundPortForward()
				fmt.Printf("Deleting KinD cluster %q...\n", clusterName)
				if err := kindDeleteCluster(clusterName); err != nil {
					return fmt.Errorf("failed to delete KinD cluster: %w", err)
				}
				if dir, err := krknctlDir(); err == nil {
					if err := os.Remove(filepath.Join(dir, clusterName+".kubeconfig")); err != nil {
						fmt.Fprintf(os.Stderr, "warning: could not remove kubeconfig: %v\n", err)
					}
				}
				fmt.Printf("KinD cluster %q deleted\n", clusterName)
				return nil
			}

			expanded, err := commonutils.ExpandFolder(kubeconfig, nil)
			if err != nil {
				return fmt.Errorf("invalid kubeconfig path: %w", err)
			}
			kubeconfigPath := *expanded

			stopBackgroundPortForward()
			fmt.Printf("Uninstalling krkn-operator from namespace %q...\n", namespace)
			if err := helmUninstall(kubeconfigPath, namespace, "krkn-operator"); err != nil {
				return fmt.Errorf("helm uninstall failed: %w", err)
			}

			if deleteNamespace {
				fmt.Printf("Deleting namespace %q...\n", namespace)
				if err := deleteK8sNamespace(kubeconfigPath, namespace); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to delete namespace %q: %v\n", namespace, err)
				}
			}

			fmt.Printf("krkn-operator uninstalled from namespace %q\n", namespace)
			return nil
		},
	}
}

func stopBackgroundPortForward() {
	dir, err := krknctlDir()
	if err != nil {
		return
	}
	pidFile := filepath.Join(dir, "port-forward.pid")
	data, err := os.ReadFile(pidFile) // #nosec G304 -- path is derived from krknctlDir() under the user's home directory, not user-supplied input
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err == nil {
		fmt.Printf("Stopped background port-forward (PID %d)\n", pid)
	}
	if err := os.Remove(pidFile); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove PID file %s: %v\n", pidFile, err)
	}
}

func krknctlDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".krknctl")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dir, err)
	}
	return dir, nil
}

func kindCreateCluster(clusterName string) (string, error) {
	fmt.Printf("Creating KinD cluster %q...\n", clusterName)

	provider := kindcluster.NewProvider(
		kindcluster.ProviderWithLogger(kindcmd.NewLogger()),
	)
	if err := provider.Create(clusterName); err != nil {
		return "", fmt.Errorf("failed to create KinD cluster: %w", err)
	}

	kubeconfigYAML, err := provider.KubeConfig(clusterName, false)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve KinD kubeconfig: %w", err)
	}

	dir, err := krknctlDir()
	if err != nil {
		return "", err
	}
	kubeconfigPath := filepath.Join(dir, clusterName+".kubeconfig")
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigYAML), 0600); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return kubeconfigPath, nil
}

func kindDeleteCluster(clusterName string) error {
	provider := kindcluster.NewProvider(
		kindcluster.ProviderWithLogger(kindcmd.NewLogger()),
	)
	return provider.Delete(clusterName, "")
}

func helmActionConfig(kubeconfigPath, namespace string) (*action.Configuration, error) {
	settings := cli.New()
	settings.KubeConfig = kubeconfigPath
	settings.SetNamespace(namespace)

	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, "secret", func(string, ...interface{}) {}); err != nil {
		return nil, fmt.Errorf("failed to initialise helm configuration: %w", err)
	}
	cfg.RegistryClient = registryClient

	return cfg, nil
}

func helmInstall(kubeconfigPath, namespace, chartRef, releaseName, version string) error {
	cfg, err := helmActionConfig(kubeconfigPath, namespace)
	if err != nil {
		return err
	}

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	client := action.NewInstall(cfg)
	client.SetRegistryClient(cfg.RegistryClient)
	client.ReleaseName = releaseName
	client.Namespace = namespace
	client.CreateNamespace = true
	client.Wait = true
	client.Timeout = 5 * time.Minute
	if version != "" {
		client.Version = version
	}

	chartPath, err := client.LocateChart(chartRef, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart %q: %w", chartRef, err)
	}

	chrt, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	if _, err := client.Run(chrt, map[string]interface{}{}); err != nil {
		return err
	}
	return nil
}

func helmUninstall(kubeconfigPath, namespace, releaseName string) error {
	cfg, err := helmActionConfig(kubeconfigPath, namespace)
	if err != nil {
		return err
	}

	client := action.NewUninstall(cfg)
	client.Wait = true

	if _, err := client.Run(releaseName); err != nil {
		return err
	}
	return nil
}

func deleteK8sNamespace(kubeconfigPath, namespace string) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	return clientset.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
}

func portForwardConsole(kubeconfigPath, namespace, svcName string, localPort, remotePort int) error {
	dir, err := krknctlDir()
	if err != nil {
		return err
	}
	pidFile := filepath.Join(dir, "port-forward.pid")

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate krknctl executable: %w", err)
	}

	// re-exec this binary with the hidden daemon subcommand so the port-forward
	// survives after the parent process exits, without depending on kubectl
	cmd := exec.Command(self, // #nosec G204 -- self-re-exec of the current binary; subcommand and all arguments are internally constructed
		"operator", "port-forward-daemon",
		"--kubeconfig", kubeconfigPath,
		"--namespace", namespace,
		"--svc", svcName,
		"--local-port", strconv.Itoa(localPort),
		"--remote-port", strconv.Itoa(remotePort),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background port-forward: %w", err)
	}

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save PID file: %v\n", err)
	}

	fmt.Printf("Console port-forward running in background (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("Console is available at http://localhost:%d\n", localPort)
	fmt.Printf("To stop: kill %d  (or: kill $(cat %s))\n", cmd.Process.Pid, pidFile)
	return nil
}

func NewOperatorPortForwardDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "port-forward-daemon",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
			namespace, _ := cmd.Flags().GetString("namespace")
			svcName, _ := cmd.Flags().GetString("svc")
			localPort, _ := cmd.Flags().GetInt("local-port")
			remotePort, _ := cmd.Flags().GetInt("remote-port")
			return runPortForwardDaemon(kubeconfigPath, namespace, svcName, localPort, remotePort)
		},
	}
	cmd.Flags().String("kubeconfig", "", "")
	cmd.Flags().String("namespace", "", "")
	cmd.Flags().String("svc", "", "")
	cmd.Flags().Int("local-port", 0, "")
	cmd.Flags().Int("remote-port", 0, "")
	return cmd
}

func runPortForwardDaemon(kubeconfigPath, namespace, svcName string, localPort, remotePort int) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	svc, err := clientset.CoreV1().Services(namespace).Get(context.Background(), svcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("service %q not found in namespace %q: %w", svcName, namespace, err)
	}

	podPort := remotePort
	for _, p := range svc.Spec.Ports {
		if p.Port == int32(remotePort) {
			podPort = p.TargetPort.IntValue()
			break
		}
	}

	selector := ""
	for k, v := range svc.Spec.Selector {
		if selector != "" {
			selector += ","
		}
		selector += k + "=" + v
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for service %q", svcName)
	}
	podName := pods.Items[0].Name

	pfURL := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return fmt.Errorf("failed to build SPDY transport: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	pf, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, podPort)},
		stopCh, readyCh,
		io.Discard, os.Stderr,
	)
	if err != nil {
		return fmt.Errorf("failed to create port-forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- pf.ForwardPorts()
	}()

	select {
	case <-readyCh:
	case err := <-errCh:
		return fmt.Errorf("port-forward failed to start: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
		close(stopCh)
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("port-forward error: %w", err)
		}
		return nil
	}
}
