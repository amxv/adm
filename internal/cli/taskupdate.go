package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/audit"
	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/identity"
	"github.com/spf13/cobra"
)

var taskUpdateCmd = &cobra.Command{
	Use:   "task-update",
	Short: "Update task description for the current agent",
	Long:  "Updates the task description and refreshes liveness for the resolved agent identity. The agent must already be registered.",
	RunE:  runTaskUpdate,
}

var taskUpdateTask string

func init() {
	taskUpdateCmd.Flags().StringVar(&taskUpdateTask, "task", "", "New task description (required)")
	_ = taskUpdateCmd.MarkFlagRequired("task")
}

func runTaskUpdate(cmd *cobra.Command, args []string) error {
	agent, err := identity.Resolve("")
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := d.Exec(`
		UPDATE agents SET task = ?, updated_at = ?, last_seen_at = ?
		WHERE name = ?
	`, taskUpdateTask, now, now, agent)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("agent %q not registered (run 'adm register --name %s --task ...' first)", agent, agent)
	}

	audit.Log(d, agent, "task-update", "", taskUpdateTask, "ok")
	fmt.Printf("task updated: %s -> %s\n", agent, taskUpdateTask)
	return nil
}
