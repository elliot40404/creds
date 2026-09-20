package cli

import (
	_ "embed"

	"github.com/spf13/cobra"
)

//go:embed agents.txt
var agentsGuide string

func agentsTopic() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "How scripts and AI agents should use creds",
		Long:  agentsGuide,
	}
}
