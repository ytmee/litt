package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive TUI issue browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			return tui.Run(s)
		},
	}
}
