package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/pathnorm"
	"github.com/spf13/cobra"
)

var checkClaimCmd = &cobra.Command{
	Use:   "check-claim",
	Short: "Check if a file is claimed by another agent",
	Long:  "Returns warning data if the file matches claims by other agents. Used by hooks before file edits.",
	RunE:  runCheckClaim,
}

var (
	checkClaimFile  string
	checkClaimAgent string
)

func init() {
	checkClaimCmd.Flags().StringVar(&checkClaimFile, "file", "", "File path to check (required)")
	checkClaimCmd.Flags().StringVar(&checkClaimAgent, "agent", "", "Calling agent name (required)")
	_ = checkClaimCmd.MarkFlagRequired("file")
	_ = checkClaimCmd.MarkFlagRequired("agent")
}

type claimWarning struct {
	Claimed bool   `json:"claimed"`
	Owner   string `json:"owner,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	File    string `json:"file"`
}

func runCheckClaim(cmd *cobra.Command, args []string) error {
	repoRoot, err := pathnorm.FindRepoRoot()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}

	normFile, err := pathnorm.Normalize(checkClaimFile, repoRoot)
	if err != nil {
		return fmt.Errorf("normalize path: %w", err)
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	// Find claims by other agents that match this file.
	rows, err := d.Query(`
		SELECT agent_name, path_pattern, path_norm
		FROM claims
		WHERE agent_name != ?
	`, checkClaimAgent)
	if err != nil {
		return fmt.Errorf("query claims: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var agentName, pattern, normPattern string
		if err := rows.Scan(&agentName, &pattern, &normPattern); err != nil {
			return fmt.Errorf("scan claim: %w", err)
		}

		if pathnorm.Match(normPattern, normFile) {
			w := claimWarning{
				Claimed: true,
				Owner:   agentName,
				Pattern: pattern,
				File:    normFile,
			}
			return json.NewEncoder(os.Stdout).Encode(w)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate claims: %w", err)
	}

	// No claim found - output minimal JSON.
	w := claimWarning{
		Claimed: false,
		File:    normFile,
	}
	return json.NewEncoder(os.Stdout).Encode(w)
}
