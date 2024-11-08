package main

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	containermanagerfactory "github.com/krkn-chaos/krknctl/pkg/container_manager/factory"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"os"
)

func main() {
	config, err := krknctlconfig.LoadConfig()
	if err != nil {
		fmt.Printf("%s\n", color.New(color.FgHiRed).Sprint("failed to load configuration"))
		os.Exit(1)
	}
	containerManagerFactory := containermanagerfactory.NewContainerManagerFactory()
	detectedRuntime, err := container_manager.DetectContainerRuntime(config)
	if err != nil {
		fmt.Printf("%s\n", color.New(color.FgHiRed).Sprint("failed to determine container runtime enviroment please install podman or docker and retry"))
		os.Exit(1)
	}

	containerManager := containerManagerFactory.NewInstance(detectedRuntime, &config)
	providerFactory := providerfactory.NewProviderFactory(&config)

	cmd.Execute(providerFactory, &containerManager, config)
}
