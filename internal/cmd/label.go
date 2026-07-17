package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels",
	}
	cmd.AddCommand(newLabelListCmd())
	return cmd
}

func newLabelListCmd() *cobra.Command {
	var jsonOutput bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			labels, err := s.ListLabels()
			if err != nil {
				return fmt.Errorf("list labels: %w", err)
			}
			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(labels); err != nil {
					return fmt.Errorf("encode labels: %w", err)
				}
				return nil
			}
			for _, l := range labels {
				cmd.Printf("%s (%s)\n", l.Name, l.Kind)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return listCmd
}
