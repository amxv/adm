package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "adm",
	Short: "Agent DM - agent-to-agent communication",
	Long:  "ADM enables coding agents to send messages, coordinate file ownership, and see who else is working on the same codebase.",
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(broadcastCmd)
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(unclaimCmd)
	rootCmd.AddCommand(checkClaimCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(inboxCmd)
}
