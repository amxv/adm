package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/db"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register an agent or update its presence",
	Long:  "Registers an agent with a name and task description. Idempotent: re-registering updates the task and refreshes liveness.",
	RunE:  runRegister,
}

var (
	registerName string
	registerTask string
)

func init() {
	registerCmd.Flags().StringVar(&registerName, "name", "", "Agent name (required)")
	registerCmd.Flags().StringVar(&registerTask, "task", "", "Description of what the agent is working on")
	_ = registerCmd.MarkFlagRequired("name")
}

func runRegister(cmd *cobra.Command, args []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := d.Exec(`
		INSERT INTO agents (name, task, created_at, updated_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			task = excluded.task,
			updated_at = excluded.updated_at,
			last_seen_at = excluded.last_seen_at
	`, registerName, registerTask, now, now, now)
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		fmt.Printf("registered: %s\n", registerName)
	}
	return nil
}
