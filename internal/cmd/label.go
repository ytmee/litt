package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels",
	}
	cmd.AddCommand(newLabelListCmd())
	cmd.AddCommand(newLabelCreateCmd())
	cmd.AddCommand(newLabelDeleteCmd())
	return cmd
}

func newLabelCreateCmd() *cobra.Command {
	var description string
	var kind string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new label",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			label, err := s.CreateLabel(args[0], description, kind)
			if err != nil {
				return fmt.Errorf("create label: %w", err)
			}
			cmd.Printf("Created label %s (%s)\n", label.Name, label.Kind)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Label description")
	cmd.Flags().StringVar(&kind, "kind", "custom", fmt.Sprintf("Label kind (%s)", strings.Join([]string{"triage", "category", "custom"}, ", ")))
	return cmd
}

func newLabelDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a label",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.DeleteLabel(args[0]); err != nil {
				return fmt.Errorf("delete label: %w", err)
			}
			cmd.Printf("Deleted label %s\n", args[0])
			return nil
		},
	}
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
