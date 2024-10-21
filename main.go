package main

import (
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	containermanagerfactory "github.com/krkn-chaos/krknctl/pkg/container_manager/factory"
	providerfactory "github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"log"
)

func main() {
	config, err := krknctlconfig.LoadConfig()
	if err != nil {
		log.Fatalf("Errore nel caricamento della configurazione: %v", err)
	}
	containerManagerFactory := containermanagerfactory.NewContainerManagerFactory()
	containerManager := containerManagerFactory.NewInstance(container_manager.DetectContainerManager(), &config)
	providerFactory := providerfactory.NewProviderFactory(&config)

	cmd.Execute(providerFactory, &containerManager, config)
}
