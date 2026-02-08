package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/audit"
	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/identity"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <agent-name>",
	Short: "Set active agent identity for this workspace",
	Long:  "Sets the active agent identity, creating a session. Registers the agent if not already registered. Subsequent mutating commands use this identity automatically without --from/--agent flags.",
	Args:  cobra.ExactArgs(1),
	RunE:  runUse,
}

var useTask string

func init() {
	useCmd.Flags().StringVar(&useTask, "task", "", "Task description (optional)")
}

func runUse(cmd *cobra.Command, args []string) error {
	name := args[0]

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	// Register/update agent.
	if useTask != "" {
		_, err = d.Exec(`
			INSERT INTO agents (name, task, created_at, updated_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				task = excluded.task,
				updated_at = excluded.updated_at,
				last_seen_at = excluded.last_seen_at
		`, name, useTask, now, now, now)
	} else {
		_, err = d.Exec(`
			INSERT INTO agents (name, task, created_at, updated_at, last_seen_at)
			VALUES (?, '', ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				updated_at = excluded.updated_at,
				last_seen_at = excluded.last_seen_at
		`, name, now, now, now)
	}
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	// Create and save session.
	sess := identity.NewSession(name)
	if err := identity.SaveSession(sess); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	audit.Log(d, name, "use", "", fmt.Sprintf("session=%s", sess.Token), "ok")

	if useTask != "" {
		fmt.Printf("active: %s (%s)\n", name, useTask)
	} else {
		fmt.Printf("active: %s\n", name)
	}
	return nil
}
