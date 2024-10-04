package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var mixCmd = &cobra.Command{
	Use:   "mix",
	Short: "make a cocktail.",
	Long:  `Mix some ingredients together to make a cocktail`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Psuedo code
		if slices.IndexFunc(
			args, func(s string) bool { return s == "Vodka" }) > -1 &&
			slices.IndexFunc(
				args, func(s string) bool { return s == "Orange Juice" }) > -1 {
			fmt.Println("You can make a screwdriver cocktail!")
			return nil
		}
		return fmt.Errorf("i can't make a cocktail from %v", args)
	},
}

func init() {
	rootCmd.AddCommand(mixCmd)
}
