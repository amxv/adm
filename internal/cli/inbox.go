package cli

import (
	"fmt"

	"github.com/amxv/adm/internal/db"
	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Read pending messages (read-only, does not change delivery state)",
	Long:  "Shows pending and offered messages for an agent. Unlike sync, this command never mutates message state. Use this for agents without hook systems.",
	RunE:  runInbox,
}

var inboxAgent string

func init() {
	inboxCmd.Flags().StringVar(&inboxAgent, "agent", "", "Agent name (required)")
	_ = inboxCmd.MarkFlagRequired("agent")
}

func runInbox(cmd *cobra.Command, args []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	rows, err := d.Query(`
		SELECT m.id, m.sender_name, m.body, m.created_at, r.state
		FROM message_receipts r
		JOIN messages m ON m.id = r.message_id
		WHERE r.recipient_name = ?
		  AND r.state IN ('pending', 'offered')
		ORDER BY r.created_at ASC
	`, inboxAgent)
	if err != nil {
		return fmt.Errorf("query inbox: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var msgID, sender, body, createdAt, state string
		if err := rows.Scan(&msgID, &sender, &body, &createdAt, &state); err != nil {
			return fmt.Errorf("scan message: %w", err)
		}

		if count == 0 {
			fmt.Println("INBOX:")
		}
		fmt.Printf("  [%s] from %s (%s): %s\n", state, sender, createdAt, body)
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate messages: %w", err)
	}

	if count == 0 {
		fmt.Println("No messages.")
	}

	return nil
}
