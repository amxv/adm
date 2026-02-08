package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/db"
	"github.com/spf13/cobra"
)

var broadcastCmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Send a message to all other agents",
	RunE:  runBroadcast,
}

var (
	broadcastFrom string
	broadcastMsg  string
)

func init() {
	broadcastCmd.Flags().StringVar(&broadcastFrom, "from", "", "Sender agent name (required)")
	broadcastCmd.Flags().StringVar(&broadcastMsg, "msg", "", "Message body (required)")
	_ = broadcastCmd.MarkFlagRequired("from")
	_ = broadcastCmd.MarkFlagRequired("msg")
}

func runBroadcast(cmd *cobra.Command, args []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	msgID := generateMsgID()

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO messages (id, sender_name, body, kind, created_at)
		VALUES (?, ?, ?, 'broadcast', ?)
	`, msgID, broadcastFrom, broadcastMsg, now)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Materialize receipts for all agents except the sender.
	rows, err := tx.Query("SELECT name FROM agents WHERE name != ?", broadcastFrom)
	if err != nil {
		return fmt.Errorf("query agents: %w", err)
	}

	var recipients []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scan agent: %w", err)
		}
		recipients = append(recipients, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate agents: %w", err)
	}

	for _, r := range recipients {
		_, err = tx.Exec(`
			INSERT INTO message_receipts (message_id, recipient_name, state, created_at)
			VALUES (?, ?, 'pending', ?)
		`, msgID, r, now)
		if err != nil {
			return fmt.Errorf("insert receipt for %s: %w", r, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("broadcast to %d agent(s)\n", len(recipients))
	return nil
}
