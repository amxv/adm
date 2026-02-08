package cli

import (
	"fmt"
	"time"

	"github.com/amxv/adm/internal/audit"
	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/identity"
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
	broadcastCmd.Flags().StringVar(&broadcastFrom, "from", "", "Sender agent name (resolved from session if omitted)")
	broadcastCmd.Flags().StringVar(&broadcastMsg, "msg", "", "Message body (required)")
	_ = broadcastCmd.MarkFlagRequired("msg")
}

func runBroadcast(cmd *cobra.Command, args []string) error {
	// Resolve sender identity.
	sender, err := identity.Resolve(broadcastFrom)
	if err != nil {
		return fmt.Errorf("sender identity: %w", err)
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	// Validate sender exists.
	var senderExists int
	err = d.QueryRow("SELECT 1 FROM agents WHERE name = ?", sender).Scan(&senderExists)
	if err != nil {
		return fmt.Errorf("sender %q not found (agents must register first)", sender)
	}

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
	`, msgID, sender, broadcastMsg, now)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Materialize receipts for all agents except the sender.
	rows, err := tx.Query("SELECT name FROM agents WHERE name != ?", sender)
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

	audit.Log(d, sender, "broadcast", "", fmt.Sprintf("msg=%s recipients=%d", msgID, len(recipients)), "ok")

	fmt.Printf("broadcast to %d agent(s)\n", len(recipients))
	return nil
}
