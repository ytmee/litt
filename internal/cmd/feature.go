package cmd

import (
	"github.com/spf13/cobra"
)

func newFeatureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage features (convenience alias for 'issue --kind feature')",
	}
	cmd.AddCommand(newFeatureCreateCmd())
	cmd.AddCommand(newFeatureListCmd())
	return cmd
}

func newFeatureCreateCmd() *cobra.Command {
	inner := newIssueCreateCmd()
	inner.Use = "create <title>"
	inner.Short = "Create a feature (convenience alias for 'issue create --kind feature')"
	inner.Long = `Create a feature.

This is a convenience alias for: litt issue create --kind feature <title>`
	origRunE := inner.RunE
	inner.RunE = func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("kind") {
			_ = cmd.Flags().Set("kind", "feature")
		}
		return origRunE(cmd, args)
	}
	return inner
}

func newFeatureListCmd() *cobra.Command {
	inner := newIssueListCmd()
	inner.Use = "list"
	inner.Short = "List features (convenience alias for 'issue list --kind feature')"
	inner.Long = `List features.

This is a convenience alias for: litt issue list --kind feature`
	origRunE := inner.RunE
	inner.RunE = func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("kind") {
			_ = cmd.Flags().Set("kind", "feature")
		}
		return origRunE(cmd, args)
	}
	return inner
}
