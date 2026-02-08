package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/pathnorm"
	"github.com/spf13/cobra"
)

var claimCmd = &cobra.Command{
	Use:   "claim <path-pattern>",
	Short: "Claim file ownership (soft signal)",
	Long:  "Declares that this agent is working on the specified files. Other agents will be warned if they try to edit claimed files.",
	Args:  cobra.ExactArgs(1),
	RunE:  runClaim,
}

var claimAgent string

func init() {
	claimCmd.Flags().StringVar(&claimAgent, "agent", "", "Agent name (required)")
	_ = claimCmd.MarkFlagRequired("agent")
}

func runClaim(cmd *cobra.Command, args []string) error {
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

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = d.Exec(`
		INSERT INTO claims (agent_name, path_pattern, path_norm, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(agent_name, path_norm) DO UPDATE SET
			path_pattern = excluded.path_pattern,
			updated_at = excluded.updated_at
	`, claimAgent, pathPattern, norm, now, now)
	if err != nil {
		return fmt.Errorf("upsert claim: %w", err)
	}

	fmt.Printf("claimed: %s -> %s\n", claimAgent, norm)
	return nil
}
