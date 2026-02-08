package cli

import (
	"fmt"

	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/pathnorm"
	"github.com/spf13/cobra"
)

var unclaimCmd = &cobra.Command{
	Use:   "unclaim <path-pattern>",
	Short: "Release file ownership claim",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnclaim,
}

var unclaimAgent string

func init() {
	unclaimCmd.Flags().StringVar(&unclaimAgent, "agent", "", "Agent name (required)")
	_ = unclaimCmd.MarkFlagRequired("agent")
}

func runUnclaim(cmd *cobra.Command, args []string) error {
	pathPattern := args[0]

	repoRoot, err := pathnorm.FindRepoRoot()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}

	norm, err := pathnorm.Normalize(pathPattern, repoRoot)
	if err != nil {
		return fmt.Errorf("normalize path: %w", err)
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	result, err := d.Exec(`
		DELETE FROM claims WHERE agent_name = ? AND path_norm = ?
	`, unclaimAgent, norm)
	if err != nil {
		return fmt.Errorf("delete claim: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("no claim found for %s on %s\n", unclaimAgent, norm)
	} else {
		fmt.Printf("unclaimed: %s -> %s\n", unclaimAgent, norm)
	}
	return nil
}
