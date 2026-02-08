package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	admdb "github.com/amxv/adm/internal/db"
)

// benchSetup creates an isolated temp directory with a .git marker for benchmarks.
func benchSetup(b *testing.B) {
	b.Helper()
	tmpDir := b.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		b.Fatal(err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.Chdir(origDir) })
}

func BenchmarkSyncEmpty(b *testing.B) {
	benchSetup(b)
	if _, err := runCmd("register", "--name", "bench-agent", "--task", "bench"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := runCmd("sync", "--agent", "bench-agent", "--format", "json"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSyncWithMessages1(b *testing.B) {
	benchSetup(b)
	if _, err := runCmd("register", "--name", "sender", "--task", "send"); err != nil {
		b.Fatal(err)
	}
	if _, err := runCmd("register", "--name", "receiver", "--task", "recv"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		runCmd("send", "--from", "sender", "--to", "receiver", "--msg", "bench")
		b.StartTimer()

		out, err := runCmd("sync", "--agent", "receiver", "--format", "json")
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		var resp syncResponse
		json.Unmarshal([]byte(out), &resp)
		if resp.BatchToken != "" {
			runCmd("sync", "--agent", "receiver", "--ack-token", resp.BatchToken, "--format", "json")
		}
		b.StartTimer()
	}
}

func BenchmarkSyncWithMessages10(b *testing.B) {
	benchSetup(b)
	if _, err := runCmd("register", "--name", "sender", "--task", "send"); err != nil {
		b.Fatal(err)
	}
	if _, err := runCmd("register", "--name", "receiver", "--task", "recv"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j := 0; j < 10; j++ {
			runCmd("send", "--from", "sender", "--to", "receiver", "--msg", fmt.Sprintf("msg-%d", j))
		}
		b.StartTimer()

		out, err := runCmd("sync", "--agent", "receiver", "--format", "json")
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		var resp syncResponse
		json.Unmarshal([]byte(out), &resp)
		if resp.BatchToken != "" {
			runCmd("sync", "--agent", "receiver", "--ack-token", resp.BatchToken, "--format", "json")
		}
		b.StartTimer()
	}
}

func BenchmarkSendDirect(b *testing.B) {
	benchSetup(b)
	if _, err := runCmd("register", "--name", "sender", "--task", "send"); err != nil {
		b.Fatal(err)
	}
	if _, err := runCmd("register", "--name", "receiver", "--task", "recv"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := runCmd("send", "--from", "sender", "--to", "receiver", "--msg", "bench"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckClaim(b *testing.B) {
	benchSetup(b)
	os.MkdirAll("src/auth", 0o755)
	os.WriteFile("src/auth/login.go", []byte("package auth"), 0o644)
	if _, err := runCmd("claim", "--agent", "alice", "src/auth"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := runCmd("check-claim", "--file", "src/auth/login.go", "--agent", "bob"); err != nil {
			b.Fatal(err)
		}
	}
}

// TestConcurrentSync verifies that multiple agents can sync simultaneously
// without lost messages or SQLite errors under concurrent access.
func TestConcurrentSync(t *testing.T) {
	testSetup(t)

	const numAgents = 24
	const msgsPerAgent = 5

	// Register agents.
	for i := 0; i < numAgents; i++ {
		if _, err := runCmd("register", "--name", fmt.Sprintf("agent-%d", i), "--task", "stress"); err != nil {
			t.Fatal(err)
		}
	}

	// Send messages: each agent sends msgsPerAgent messages to the next agent.
	for i := 0; i < numAgents; i++ {
		from := fmt.Sprintf("agent-%d", i)
		to := fmt.Sprintf("agent-%d", (i+1)%numAgents)
		for j := 0; j < msgsPerAgent; j++ {
			if _, err := runCmd("send", "--from", from, "--to", to, "--msg", fmt.Sprintf("m%d", j)); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Concurrently sync all agents using direct DB access.
	// runCmd is not goroutine-safe (global flags, stdout redirect), so we
	// open the DB directly and execute the sync SQL in each goroutine.
	var wg sync.WaitGroup
	delivered := make([]int, numAgents)
	errs := make([]error, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("agent-%d", idx)
			ackToken := ""

			for round := 0; round < 20; round++ {
				d, err := admdb.Open()
				if err != nil {
					errs[idx] = fmt.Errorf("open: %w", err)
					return
				}

				now := time.Now().UTC().Format(time.RFC3339)
				ctx := context.Background()

				conn, err := d.Conn(ctx)
				if err != nil {
					d.Close()
					errs[idx] = fmt.Errorf("conn: %w", err)
					return
				}

				if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
					conn.Close()
					d.Close()
					errs[idx] = fmt.Errorf("begin: %w", err)
					return
				}

				// Heartbeat.
				conn.ExecContext(ctx, "UPDATE agents SET last_seen_at=?, updated_at=? WHERE name=?", now, now, name)

				// Ack previous batch.
				if ackToken != "" {
					conn.ExecContext(ctx, "UPDATE message_receipts SET state='delivered', delivered_at=? WHERE recipient_name=? AND batch_token=? AND state='offered'", now, name, ackToken)
				}

				// Fetch pending receipts.
				rows, err := conn.QueryContext(ctx, "SELECT r.id FROM message_receipts r WHERE r.recipient_name=? AND r.state='pending' ORDER BY r.created_at ASC LIMIT 10", name)
				var ids []int64
				if err == nil {
					for rows.Next() {
						var id int64
						rows.Scan(&id)
						ids = append(ids, id)
					}
					rows.Close()
				}

				// Offer batch.
				newToken := ""
				if len(ids) > 0 {
					b := make([]byte, 8)
					rand.Read(b)
					newToken = fmt.Sprintf("bat_%x", b)
					conn.ExecContext(ctx, "INSERT INTO sync_batches (token, agent_name, created_at) VALUES (?,?,?)", newToken, name, now)
					for _, id := range ids {
						conn.ExecContext(ctx, "UPDATE message_receipts SET state='offered', offered_at=?, batch_token=? WHERE id=?", now, newToken, id)
					}
				}

				conn.ExecContext(ctx, "COMMIT")
				conn.Close()
				d.Close()

				delivered[idx] += len(ids)
				ackToken = newToken

				if len(ids) == 0 && ackToken == "" {
					break
				}
			}
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("agent-%d: %v", i, err)
		}
	}

	total := 0
	for _, d := range delivered {
		total += d
	}

	expected := numAgents * msgsPerAgent
	if total != expected {
		t.Errorf("delivery mismatch: sent %d, received %d (per-agent: %v)", expected, total, delivered)
	}
}

// TestLatencyReport runs key operations and reports p50/p95 latencies.
// Run with -v to see the report. Skipped in short mode.
func TestLatencyReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency report in short mode")
	}

	testSetup(t)

	// Setup.
	runCmd("register", "--name", "sender", "--task", "send")
	runCmd("register", "--name", "receiver", "--task", "recv")
	os.MkdirAll("src/auth", 0o755)
	os.WriteFile("src/auth/login.go", []byte("package auth"), 0o644)
	runCmd("claim", "--agent", "owner", "src/auth")

	const N = 50

	// Sync empty.
	durations := measureN(N, func() {
		runCmd("sync", "--agent", "receiver", "--format", "json")
	})
	reportPercentiles(t, "sync empty", durations)

	// Send direct.
	durations = measureN(N, func() {
		runCmd("send", "--from", "sender", "--to", "receiver", "--msg", "bench")
	})
	reportPercentiles(t, "send direct", durations)

	// Drain messages from send benchmark above.
	for {
		out, _ := runCmd("sync", "--agent", "receiver", "--format", "json")
		var resp syncResponse
		json.Unmarshal([]byte(out), &resp)
		if resp.BatchToken == "" {
			break
		}
		runCmd("sync", "--agent", "receiver", "--ack-token", resp.BatchToken, "--format", "json")
	}

	// Sync with 1 message.
	durations = make([]time.Duration, N)
	for i := 0; i < N; i++ {
		runCmd("send", "--from", "sender", "--to", "receiver", "--msg", "bench")
		start := time.Now()
		out, _ := runCmd("sync", "--agent", "receiver", "--format", "json")
		durations[i] = time.Since(start)
		var resp syncResponse
		json.Unmarshal([]byte(out), &resp)
		if resp.BatchToken != "" {
			runCmd("sync", "--agent", "receiver", "--ack-token", resp.BatchToken, "--format", "json")
		}
	}
	reportPercentiles(t, "sync with 1 message", durations)

	// Check-claim.
	durations = measureN(N, func() {
		runCmd("check-claim", "--file", "src/auth/login.go", "--agent", "bob")
	})
	reportPercentiles(t, "check-claim", durations)
}

func measureN(n int, fn func()) []time.Duration {
	durations := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		durations[i] = time.Since(start)
	}
	return durations
}

func reportPercentiles(t *testing.T, label string, durations []time.Duration) {
	t.Helper()
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	n := len(durations)
	p50 := durations[n/2]
	p95 := durations[int(float64(n)*0.95)]
	t.Logf("%-25s p50=%-12v p95=%-12v", label, p50, p95)
}
