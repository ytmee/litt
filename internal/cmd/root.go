package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "litt",
		Short: "Local-first task graph and execution tracker for AI agents",
	}
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLabelCmd())
	cmd.AddCommand(newAgentCmd())
	return cmd
}
