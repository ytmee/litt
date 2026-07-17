package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed agent-instructions.md
var agentInstructionsContent string

const managedBlockStart = "<!-- litt-agent-instructions-start -->\n"
const managedBlockEnd = "\n<!-- litt-agent-instructions-end -->"

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent integration commands",
	}
	cmd.AddCommand(newAgentInstructionsCmd())
	cmd.AddCommand(newAgentInstallCmd())
	return cmd
}

func managedBlock() string {
	return managedBlockStart + strings.TrimSpace(managedContent()) + managedBlockEnd
}

func managedContent() string {
	return agentInstructionsContent
}

func newAgentInstructionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "instructions",
		Short: "Print the managed agent instructions block",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(managedBlock())
			return nil
		},
	}
}

func newAgentInstallCmd() *cobra.Command {
	var target string
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install agent instructions into AGENTS.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(target)
			if err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("read %s: %w", target, err)
				}
				content = []byte{}
			}
			block := managedBlock()
			text := string(content)
			startIdx := strings.Index(text, managedBlockStart)
			if startIdx >= 0 {
				endIdx := strings.Index(text, managedBlockEnd)
				if endIdx < 0 {
					endIdx = len(text)
				} else {
					endIdx += len(managedBlockEnd)
				}
				text = text[:startIdx] + block + text[endIdx:]
			} else {
				if len(text) > 0 && text[len(text)-1] != '\n' {
					text += "\n"
				}
				text += "\n" + block + "\n"
			}
			if err := os.WriteFile(target, []byte(text), 0644); err != nil {
				return fmt.Errorf("write %s: %w", target, err)
			}
			cmd.Printf("Installed agent instructions into %s\n", target)
			return nil
		},
	}
	installCmd.Flags().StringVar(&target, "target", "AGENTS.md", "Target file path")
	return installCmd
}
