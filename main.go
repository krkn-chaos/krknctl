package main

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/cmd"
	krknctlconfig "github.com/krkn-chaos/krknctl/internal/config"
	"log"
)

func main() {
	config, err := krknctlconfig.LoadConfig()
	if err != nil {
		log.Fatalf("Errore nel caricamento della configurazione: %v", err)
	}
	fmt.Print("config: " + config.AppName)
	cmd.Execute()
}
