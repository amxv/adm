package cli

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/amxv/adm/internal/audit"
	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/identity"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a direct message to another agent",
	RunE:  runSend,
}

var (
	sendFrom string
	sendTo   string
	sendMsg  string
)

func init() {
	sendCmd.Flags().StringVar(&sendFrom, "from", "", "Sender agent name (resolved from session if omitted)")
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Recipient agent name (required)")
	sendCmd.Flags().StringVar(&sendMsg, "msg", "", "Message body (required)")
	_ = sendCmd.MarkFlagRequired("to")
	_ = sendCmd.MarkFlagRequired("msg")
}

func runSend(cmd *cobra.Command, args []string) error {
	// Resolve sender identity.
	sender, err := identity.Resolve(sendFrom)
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

	// Validate recipient exists.
	var exists int
	err = d.QueryRow("SELECT 1 FROM agents WHERE name = ?", sendTo).Scan(&exists)
	if err != nil {
		return fmt.Errorf("recipient %q not found (agents must register first)", sendTo)
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
		VALUES (?, ?, ?, 'direct', ?)
	`, msgID, sender, sendMsg, now)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO message_receipts (message_id, recipient_name, state, created_at)
		VALUES (?, ?, 'pending', ?)
	`, msgID, sendTo, now)
	if err != nil {
		return fmt.Errorf("insert receipt: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	audit.Log(d, sender, "send", sendTo, fmt.Sprintf("msg=%s", msgID), "ok")

	fmt.Printf("sent to %s\n", sendTo)
	return nil
}

// generateMsgID creates a short random message ID prefixed with "msg_".
func generateMsgID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("msg_%x", b)
}
