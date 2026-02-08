package cli

import (
	"fmt"
	"os"

	"github.com/amxv/adm/internal/db"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative maintenance commands (requires ADM_ADMIN=1)",
	Long:  "Maintenance commands for managing ADM state. Requires the ADM_ADMIN=1 environment variable to be set.",
}

func init() {
	adminCmd.AddCommand(adminPurgeCmd)
	adminCmd.AddCommand(adminAuditLogCmd)
}

func requireAdmin() error {
	if os.Getenv("ADM_ADMIN") != "1" {
		return fmt.Errorf("admin commands require ADM_ADMIN=1 environment variable")
	}
	return nil
}

// --- purge-delivered ---

var adminPurgeCmd = &cobra.Command{
	Use:   "purge-delivered",
	Short: "Purge delivered messages older than N days",
	RunE:  runAdminPurge,
}

var purgeDays int

func init() {
	adminPurgeCmd.Flags().IntVar(&purgeDays, "days", 7, "Purge messages delivered more than N days ago")
}

func runAdminPurge(cmd *cobra.Command, args []string) error {
	if err := requireAdmin(); err != nil {
		return err
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	cutoff := fmt.Sprintf("-%d days", purgeDays)

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete receipts for fully-delivered messages older than cutoff.
	result, err := tx.Exec(`
		DELETE FROM message_receipts
		WHERE message_id IN (
			SELECT m.id FROM messages m
			WHERE m.created_at < datetime('now', ?)
			AND NOT EXISTS (
				SELECT 1 FROM message_receipts r
				WHERE r.message_id = m.id AND r.state != 'delivered'
			)
		)
	`, cutoff)
	if err != nil {
		return fmt.Errorf("purge receipts: %w", err)
	}
	receiptRows, _ := result.RowsAffected()

	// Delete orphaned messages (no remaining receipts).
	result, err = tx.Exec(`
		DELETE FROM messages
		WHERE created_at < datetime('now', ?)
		AND NOT EXISTS (
			SELECT 1 FROM message_receipts r
			WHERE r.message_id = messages.id
		)
	`, cutoff)
	if err != nil {
		return fmt.Errorf("purge messages: %w", err)
	}
	msgRows, _ := result.RowsAffected()

	// Purge old sync batches.
	result, err = tx.Exec(`
		DELETE FROM sync_batches WHERE created_at < datetime('now', ?)
	`, cutoff)
	if err != nil {
		return fmt.Errorf("purge batches: %w", err)
	}
	batchRows, _ := result.RowsAffected()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("purged: %d message(s), %d receipt(s), %d batch(es) older than %d day(s)\n",
		msgRows, receiptRows, batchRows, purgeDays)
	return nil
}

// --- audit-log ---

var adminAuditLogCmd = &cobra.Command{
	Use:   "audit-log",
	Short: "View recent audit log entries",
	RunE:  runAdminAuditLog,
}

var auditLogLimit int

func init() {
	adminAuditLogCmd.Flags().IntVar(&auditLogLimit, "limit", 50, "Number of entries to show")
}

func runAdminAuditLog(cmd *cobra.Command, args []string) error {
	if err := requireAdmin(); err != nil {
		return err
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	rows, err := d.Query(`
		SELECT agent_name, action, target, detail, outcome, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT ?
	`, auditLogLimit)
	if err != nil {
		return fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var agent, action, target, detail, outcome, createdAt string
		if err := rows.Scan(&agent, &action, &target, &detail, &outcome, &createdAt); err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		if count == 0 {
			fmt.Println("AUDIT LOG:")
		}
		line := fmt.Sprintf("  %s  %-10s %-10s", createdAt, agent, action)
		if target != "" {
			line += fmt.Sprintf("  target=%s", target)
		}
		if detail != "" {
			line += fmt.Sprintf("  %s", detail)
		}
		if outcome != "ok" {
			line += fmt.Sprintf("  [%s]", outcome)
		}
		fmt.Println(line)
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	if count == 0 {
		fmt.Println("No audit log entries.")
	}
	return nil
}
