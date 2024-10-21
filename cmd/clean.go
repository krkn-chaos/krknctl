package cmd

import (
	"fmt"
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/container_manager"
	"github.com/spf13/cobra"
)

func NewCleanCommand(containerManager *container_manager.ContainerManager, config config.Config) *cobra.Command {
	var cleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "cleans already run scenario files and containers",
		Long:  `cleans already run scenario files and containers`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			deletedContainers, err := (*containerManager).CleanContainers()
			if err != nil {
				return err
			}
			deletedFiles, err := container_manager.CleanKubeconfigFiles(config)
			if err != nil {
				return err
			}
			fmt.Println(fmt.Sprintf("%d containers, %d kubeconfig files deleted", *deletedContainers, *deletedFiles))
			return nil
		},
	}

	return cleanCmd
}
