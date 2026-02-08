package audit

import (
	"database/sql"
	"time"
)

// Log records a mutating operation in the audit log.
// Best-effort: audit failures do not propagate to the caller.
func Log(db *sql.DB, agent, action, target, detail, outcome string) {
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`
		INSERT INTO audit_log (agent_name, action, target, detail, outcome, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, agent, action, target, detail, outcome, now)
}
