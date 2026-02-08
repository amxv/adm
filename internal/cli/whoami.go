package cli

import (
	"fmt"

	"github.com/amxv/adm/internal/identity"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current agent identity",
	Long:  "Prints the resolved agent identity from session, environment, or legacy agent file.",
	RunE:  runWhoami,
}

func runWhoami(cmd *cobra.Command, args []string) error {
	agent, err := identity.Resolve("")
	if err != nil {
		return err
	}
	fmt.Println(agent)
	return nil
}
