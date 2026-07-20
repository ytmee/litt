package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "litt",
		Short:   "Local-first task graph and execution tracker for AI agents",
		Version: version,
	}
	cmd.PersistentFlags().String("db", "", "Path to litt database (auto-detected by default)")
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newIssueCmd())
	cmd.AddCommand(newLabelCmd())
	cmd.AddCommand(newAgentCmd())
	cmd.AddCommand(newMCPCmd())
	cmd.AddCommand(newTUICmd())
	return cmd
}
