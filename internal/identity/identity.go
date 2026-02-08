package identity

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amxv/adm/internal/pathnorm"
)

// Session represents an active agent session in this workspace.
type Session struct {
	Agent     string `json:"agent"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

// SessionPath returns the path to the session file.
func SessionPath() (string, error) {
	root, err := pathnorm.FindRepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".agents", "adm", "state", "session.json"), nil
}

// LoadSession reads the current session from disk.
func LoadSession() (*Session, error) {
	path, err := SessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no active session (run 'adm use <name>'): %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid session file: %w", err)
	}
	if s.Agent == "" {
		return nil, fmt.Errorf("session file missing agent name")
	}
	return &s, nil
}

// SaveSession writes a session to disk, creating directories as needed.
func SaveSession(s *Session) error {
	path, err := SessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// NewSession creates a new session for the given agent.
func NewSession(agent string) *Session {
	b := make([]byte, 16)
	rand.Read(b)
	return &Session{
		Agent:     agent,
		Token:     fmt.Sprintf("ses_%x", b),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// Resolve resolves the agent identity from available sources.
// Priority: explicitFlag > ADM_AGENT env var > session file > .agents/adm/agent file.
// Returns the resolved agent name, or an error if no identity is found.
func Resolve(explicitFlag string) (string, error) {
	if explicitFlag != "" {
		return strings.TrimSpace(explicitFlag), nil
	}

	if env := os.Getenv("ADM_AGENT"); env != "" {
		return strings.TrimSpace(env), nil
	}

	if sess, err := LoadSession(); err == nil && sess.Agent != "" {
		return sess.Agent, nil
	}

	// Legacy fallback: .agents/adm/agent file.
	root, err := pathnorm.FindRepoRoot()
	if err == nil {
		agentFile := filepath.Join(root, ".agents", "adm", "agent")
		data, err := os.ReadFile(agentFile)
		if err == nil {
			name := strings.TrimSpace(string(data))
			if name != "" {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("no agent identity found (run 'adm use <name>' or set ADM_AGENT)")
}
