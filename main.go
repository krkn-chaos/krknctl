package main

import (
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider/factory"
	"log"
)

func main() {
	config, err := krknctlconfig.LoadConfig()
	if err != nil {
		log.Fatalf("Errore nel caricamento della configurazione: %v", err)
	}
	providerFactory := factory.NewProviderFactory(&config)
	cmd.Execute(providerFactory, config)
}
