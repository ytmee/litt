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
	cmd.AddCommand(newIssueCmd())
	cmd.AddCommand(newLabelCmd())
	cmd.AddCommand(newAgentCmd())
	cmd.AddCommand(newMCPCmd())
	return cmd
}
