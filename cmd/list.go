package cmd

import (
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"time"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List scenarios",
	Long:  `List available krkn-hub scenarios`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := spinner.New(spinner.CharSets[39], 100*time.Millisecond)
		s.Suffix = "fetching scenarios"
		s.Start()
		time.Sleep(4 * time.Second)
		s.Stop()
		if offlineFlag {
			fmt.Println("OFFLINE MODE")
		}
		if jsonFlag {
			fmt.Println("{\"hello\":\"list\"}")
			return nil
		} else {
			fmt.Println("Listing scenarios...")
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
