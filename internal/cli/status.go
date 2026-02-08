package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/db"
	"github.com/spf13/cobra"
)

const staleTTL = 5 * time.Minute

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show registered agents and their status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	rows, err := d.Query(`
		SELECT name, task, last_seen_at
		FROM agents
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	count := 0

	for rows.Next() {
		var name, task, lastSeenStr string
		if err := rows.Scan(&name, &task, &lastSeenStr); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		lastSeen, err := time.Parse(time.RFC3339, lastSeenStr)
		if err != nil {
			return fmt.Errorf("parse last_seen_at: %w", err)
		}

		state := "online"
		if now.Sub(lastSeen) > staleTTL {
			state = "stale"
		}

		if count == 0 {
			fmt.Println("AGENTS:")
		}
		if task != "" {
			fmt.Printf("  %-12s [%s]  %s\n", name, state, task)
		} else {
			fmt.Printf("  %-12s [%s]\n", name, state)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	if count == 0 {
		fmt.Println("No agents registered.")
	}

	return nil
}
