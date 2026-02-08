package cli

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amxv/adm/internal/audit"
	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/identity"
	"github.com/spf13/cobra"
)

const maxSyncBatch = 10

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync messages for an agent (called by hooks)",
	Long:  "Returns unread messages as JSON and manages delivery acknowledgement via batch tokens. Designed to be called by hook scripts.",
	RunE:  runSync,
}

var (
	syncAgent    string
	syncAckToken string
	syncFormat   string
)

func init() {
	syncCmd.Flags().StringVar(&syncAgent, "agent", "", "Agent name (resolved from session if omitted)")
	syncCmd.Flags().StringVar(&syncAckToken, "ack-token", "", "Previous batch token to acknowledge")
	syncCmd.Flags().StringVar(&syncFormat, "format", "json", "Output format (json)")
}

type syncResponse struct {
	Messages   []syncMessage `json:"messages"`
	BatchToken string        `json:"batch_token"`
}

type syncMessage struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func runSync(cmd *cobra.Command, args []string) error {
	// Resolve agent identity.
	agent, err := identity.Resolve(syncAgent)
	if err != nil {
		return fmt.Errorf("agent identity: %w", err)
	}

	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	ctx := cmd.Context()

	// Use a raw connection with BEGIN IMMEDIATE to acquire a RESERVED lock
	// upfront, avoiding lock-promotion failures under concurrent writers.
	conn, err := d.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin immediate: %w", err)
	}
	defer conn.ExecContext(ctx, "ROLLBACK") // no-op after successful commit

	// 1) Heartbeat: update last_seen_at.
	_, err = conn.ExecContext(ctx, `
		UPDATE agents SET last_seen_at = ?, updated_at = ? WHERE name = ?
	`, now, now, agent)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}

	// 2) Acknowledge previous batch if token provided.
	if syncAckToken != "" {
		_, err = conn.ExecContext(ctx, `
			UPDATE message_receipts
			SET state = 'delivered', delivered_at = ?
			WHERE recipient_name = ?
			  AND batch_token = ?
			  AND state = 'offered'
		`, now, agent, syncAckToken)
		if err != nil {
			return fmt.Errorf("ack batch: %w", err)
		}
	}

	// 3) Select next pending messages.
	rows, err := conn.QueryContext(ctx, `
		SELECT r.id, m.id, m.sender_name, r.recipient_name, m.body, m.created_at
		FROM message_receipts r
		JOIN messages m ON m.id = r.message_id
		WHERE r.recipient_name = ?
		  AND r.state = 'pending'
		ORDER BY r.created_at ASC
		LIMIT ?
	`, agent, maxSyncBatch)
	if err != nil {
		return fmt.Errorf("query pending: %w", err)
	}

	type receiptRow struct {
		receiptID     int64
		msgID         string
		senderName    string
		recipientName string
		body          string
		createdAt     string
	}

	var pending []receiptRow
	for rows.Next() {
		var r receiptRow
		if err := rows.Scan(&r.receiptID, &r.msgID, &r.senderName, &r.recipientName, &r.body, &r.createdAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan receipt: %w", err)
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate receipts: %w", err)
	}

	// 4) If messages found, create batch and mark as offered.
	var batchToken string
	if len(pending) > 0 {
		batchToken = generateBatchToken()

		_, err = conn.ExecContext(ctx, `
			INSERT INTO sync_batches (token, agent_name, created_at)
			VALUES (?, ?, ?)
		`, batchToken, agent, now)
		if err != nil {
			return fmt.Errorf("insert batch: %w", err)
		}

		// Build ID list for UPDATE.
		ids := make([]string, len(pending))
		ifaces := make([]interface{}, len(pending)+2)
		ifaces[0] = now
		ifaces[1] = batchToken
		for i, r := range pending {
			ids[i] = "?"
			ifaces[i+2] = r.receiptID
		}

		query := fmt.Sprintf(`
			UPDATE message_receipts
			SET state = 'offered', offered_at = ?, batch_token = ?
			WHERE id IN (%s)
		`, strings.Join(ids, ","))

		_, err = conn.ExecContext(ctx, query, ifaces...)
		if err != nil {
			return fmt.Errorf("mark offered: %w", err)
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Audit: log sync with summary (only for non-empty syncs).
	if syncAckToken != "" || len(pending) > 0 {
		detail := fmt.Sprintf("offered=%d", len(pending))
		if syncAckToken != "" {
			detail += fmt.Sprintf(" acked=%s", syncAckToken)
		}
		audit.Log(d, agent, "sync", "", detail, "ok")
	}

	// Build and output response.
	resp := syncResponse{
		Messages:   make([]syncMessage, len(pending)),
		BatchToken: batchToken,
	}
	for i, r := range pending {
		resp.Messages[i] = syncMessage{
			ID:        r.msgID,
			From:      r.senderName,
			To:        r.recipientName,
			Body:      r.body,
			CreatedAt: r.createdAt,
		}
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

func generateBatchToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("bat_%x", b)
}
