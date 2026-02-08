package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amxv/adm/internal/pathnorm"
)

const (
	defaultLimit = 50
	maxLimit     = 500
	staleTTL     = 5 * time.Minute
)

// --- Health ---

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

var buildVersion = "dev"

// SetVersion sets the version reported by /api/v1/health.
func SetVersion(v string) {
	buildVersion = v
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: buildVersion,
	})
}

// --- Agents ---

type agentRow struct {
	Name       string `json:"name"`
	Task       string `json:"task"`
	Status     string `json:"status"`
	LastSeenAt string `json:"last_seen_at"`
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT name, task, last_seen_at
		FROM agents
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query agents: "+err.Error())
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	agents := []agentRow{}

	for rows.Next() {
		var a agentRow
		if err := rows.Scan(&a.Name, &a.Task, &a.LastSeenAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan agent: "+err.Error())
			return
		}
		lastSeen, err := time.Parse(time.RFC3339, a.LastSeenAt)
		if err != nil {
			a.Status = "unknown"
		} else if now.Sub(lastSeen) > staleTTL {
			a.Status = "stale"
		} else {
			a.Status = "online"
		}
		agents = append(agents, a)
	}

	writeJSON(w, http.StatusOK, agents)
}

// --- Claims ---

type claimRow struct {
	AgentName   string `json:"agent_name"`
	PathPattern string `json:"path_pattern"`
	PathNorm    string `json:"path_norm"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Server) handleClaims(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT agent_name, path_pattern, path_norm, created_at, updated_at
		FROM claims
		ORDER BY agent_name, path_norm
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query claims: "+err.Error())
		return
	}
	defer rows.Close()

	claims := []claimRow{}
	for rows.Next() {
		var c claimRow
		if err := rows.Scan(&c.AgentName, &c.PathPattern, &c.PathNorm, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan claim: "+err.Error())
			return
		}
		claims = append(claims, c)
	}

	writeJSON(w, http.StatusOK, claims)
}

// --- Messages ---

type deliverySummary struct {
	Pending   int `json:"pending"`
	Offered   int `json:"offered"`
	Delivered int `json:"delivered"`
	Total     int `json:"total"`
}

type messageRow struct {
	ID         string          `json:"id"`
	From       string          `json:"from"`
	Kind       string          `json:"kind"`
	Body       string          `json:"body"`
	CreatedAt  string          `json:"created_at"`
	Recipients []string        `json:"recipients"`
	Delivery   deliverySummary `json:"delivery"`
}

type pageInfo struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type messagesResponse struct {
	Items []messageRow `json:"items"`
	Page  pageInfo     `json:"page"`
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := parseIntParam(q.Get("limit"), defaultLimit)
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := parseIntParam(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	// Build WHERE clauses based on filters.
	var conditions []string
	var args []any

	if v := q.Get("q"); v != "" {
		conditions = append(conditions, "m.body LIKE ?")
		args = append(args, "%"+v+"%")
	}
	if v := q.Get("from"); v != "" {
		conditions = append(conditions, "m.sender_name = ?")
		args = append(args, v)
	}
	if v := q.Get("kind"); v != "" {
		conditions = append(conditions, "m.kind = ?")
		args = append(args, v)
	}
	if v := q.Get("from_ts"); v != "" {
		conditions = append(conditions, "m.created_at >= ?")
		args = append(args, v)
	}
	if v := q.Get("to_ts"); v != "" {
		conditions = append(conditions, "m.created_at <= ?")
		args = append(args, v)
	}

	// Recipient filter ("to") requires a subquery.
	if v := q.Get("to"); v != "" {
		conditions = append(conditions, "m.id IN (SELECT message_id FROM message_receipts WHERE recipient_name = ?)")
		args = append(args, v)
	}

	// State filter requires a subquery.
	if v := q.Get("state"); v != "" {
		conditions = append(conditions, "m.id IN (SELECT message_id FROM message_receipts WHERE state = ?)")
		args = append(args, v)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching messages.
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM messages m %s", whereClause)
	var total int
	if err := s.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "count messages: "+err.Error())
		return
	}

	// Fetch paginated messages.
	dataQuery := fmt.Sprintf(`
		SELECT m.id, m.sender_name, m.kind, m.body, m.created_at
		FROM messages m
		%s
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	dataArgs := make([]any, len(args)+2)
	copy(dataArgs, args)
	dataArgs[len(args)] = limit
	dataArgs[len(args)+1] = offset

	rows, err := s.db.QueryContext(r.Context(), dataQuery, dataArgs...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query messages: "+err.Error())
		return
	}
	defer rows.Close()

	items := []messageRow{}
	var msgIDs []string

	for rows.Next() {
		var m messageRow
		if err := rows.Scan(&m.ID, &m.From, &m.Kind, &m.Body, &m.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan message: "+err.Error())
			return
		}
		items = append(items, m)
		msgIDs = append(msgIDs, m.ID)
	}

	// Fetch receipt data for all messages in one query.
	if len(msgIDs) > 0 {
		placeholders := make([]string, len(msgIDs))
		receiptArgs := make([]any, len(msgIDs))
		for i, id := range msgIDs {
			placeholders[i] = "?"
			receiptArgs[i] = id
		}

		receiptQuery := fmt.Sprintf(`
			SELECT message_id, recipient_name, state
			FROM message_receipts
			WHERE message_id IN (%s)
		`, strings.Join(placeholders, ","))

		rRows, err := s.db.QueryContext(r.Context(), receiptQuery, receiptArgs...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query receipts: "+err.Error())
			return
		}
		defer rRows.Close()

		// Build lookup maps.
		type receiptInfo struct {
			recipient string
			state     string
		}
		receiptsByMsg := make(map[string][]receiptInfo)

		for rRows.Next() {
			var msgID, recipient, state string
			if err := rRows.Scan(&msgID, &recipient, &state); err != nil {
				writeError(w, http.StatusInternalServerError, "scan receipt: "+err.Error())
				return
			}
			receiptsByMsg[msgID] = append(receiptsByMsg[msgID], receiptInfo{recipient, state})
		}

		// Populate items with receipt data.
		for i := range items {
			receipts := receiptsByMsg[items[i].ID]
			seen := make(map[string]bool)
			var ds deliverySummary
			for _, ri := range receipts {
				if !seen[ri.recipient] {
					items[i].Recipients = append(items[i].Recipients, ri.recipient)
					seen[ri.recipient] = true
				}
				ds.Total++
				switch ri.state {
				case "pending":
					ds.Pending++
				case "offered":
					ds.Offered++
				case "delivered":
					ds.Delivered++
				}
			}
			items[i].Delivery = ds
			if items[i].Recipients == nil {
				items[i].Recipients = []string{}
			}
		}
	}

	writeJSON(w, http.StatusOK, messagesResponse{
		Items: items,
		Page: pageInfo{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

// --- Message Detail ---

type receiptDetail struct {
	Recipient   string `json:"recipient"`
	State       string `json:"state"`
	BatchToken  string `json:"batch_token,omitempty"`
	OfferedAt   string `json:"offered_at,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
}

type messageDetail struct {
	ID        string          `json:"id"`
	From      string          `json:"from"`
	Kind      string          `json:"kind"`
	Body      string          `json:"body"`
	CreatedAt string          `json:"created_at"`
	Receipts  []receiptDetail `json:"receipts"`
}

func (s *Server) handleMessageDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing message id")
		return
	}

	var msg messageDetail
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id, sender_name, kind, body, created_at
		FROM messages
		WHERE id = ?
	`, id).Scan(&msg.ID, &msg.From, &msg.Kind, &msg.Body, &msg.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT recipient_name, state, COALESCE(batch_token, ''), COALESCE(offered_at, ''), COALESCE(delivered_at, '')
		FROM message_receipts
		WHERE message_id = ?
		ORDER BY recipient_name
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query receipts: "+err.Error())
		return
	}
	defer rows.Close()

	msg.Receipts = []receiptDetail{}
	for rows.Next() {
		var rd receiptDetail
		if err := rows.Scan(&rd.Recipient, &rd.State, &rd.BatchToken, &rd.OfferedAt, &rd.DeliveredAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan receipt: "+err.Error())
			return
		}
		msg.Receipts = append(msg.Receipts, rd)
	}

	writeJSON(w, http.StatusOK, msg)
}

// --- Claim Conflicts ---

type conflictPair struct {
	ClaimA      claimRow `json:"claim_a"`
	ClaimB      claimRow `json:"claim_b"`
	OverlapType string   `json:"overlap_type"`
}

type conflictsResponse struct {
	Conflicts []conflictPair `json:"conflicts"`
	Total     int            `json:"total"`
}

func (s *Server) handleClaimConflicts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT agent_name, path_pattern, path_norm, created_at, updated_at
		FROM claims
		ORDER BY agent_name, path_norm
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query claims: "+err.Error())
		return
	}
	defer rows.Close()

	var claims []claimRow
	for rows.Next() {
		var c claimRow
		if err := rows.Scan(&c.AgentName, &c.PathPattern, &c.PathNorm, &c.CreatedAt, &c.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan claim: "+err.Error())
			return
		}
		claims = append(claims, c)
	}

	var conflicts []conflictPair
	for i := 0; i < len(claims); i++ {
		for j := i + 1; j < len(claims); j++ {
			if claims[i].AgentName == claims[j].AgentName {
				continue
			}
			aMatchesB := pathnorm.Match(claims[i].PathNorm, claims[j].PathNorm)
			bMatchesA := pathnorm.Match(claims[j].PathNorm, claims[i].PathNorm)
			if aMatchesB || bMatchesA {
				overlapType := "glob"
				if claims[i].PathNorm == claims[j].PathNorm {
					overlapType = "exact"
				} else if aMatchesB && bMatchesA {
					overlapType = "mutual"
				} else {
					overlapType = "subset"
				}
				conflicts = append(conflicts, conflictPair{
					ClaimA:      claims[i],
					ClaimB:      claims[j],
					OverlapType: overlapType,
				})
			}
		}
	}

	if conflicts == nil {
		conflicts = []conflictPair{}
	}

	writeJSON(w, http.StatusOK, conflictsResponse{
		Conflicts: conflicts,
		Total:     len(conflicts),
	})
}

// --- Delivery Debug ---

type deliveryDebugResponse struct {
	Receipts     deliverySummary    `json:"receipts"`
	RecentBatches []syncBatchRow    `json:"recent_batches"`
}

type syncBatchRow struct {
	Token     string `json:"token"`
	AgentName string `json:"agent_name"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) handleDeliveryDebug(w http.ResponseWriter, r *http.Request) {
	// Aggregate receipt counts.
	var ds deliverySummary
	err := s.db.QueryRowContext(r.Context(), `
		SELECT
			COALESCE(SUM(CASE WHEN state = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN state = 'offered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN state = 'delivered' THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM message_receipts
	`).Scan(&ds.Pending, &ds.Offered, &ds.Delivered, &ds.Total)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query receipts: "+err.Error())
		return
	}

	// Recent sync batches.
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT token, agent_name, created_at
		FROM sync_batches
		ORDER BY created_at DESC
		LIMIT 20
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query batches: "+err.Error())
		return
	}
	defer rows.Close()

	batches := []syncBatchRow{}
	for rows.Next() {
		var b syncBatchRow
		if err := rows.Scan(&b.Token, &b.AgentName, &b.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan batch: "+err.Error())
			return
		}
		batches = append(batches, b)
	}

	writeJSON(w, http.StatusOK, deliveryDebugResponse{
		Receipts:      ds,
		RecentBatches: batches,
	})
}

// --- Helpers ---

func parseIntParam(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
