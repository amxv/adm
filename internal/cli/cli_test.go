package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testSetup creates an isolated temp directory with a .git marker so
// the DB is created in a predictable location. It changes to the temp
// dir and restores the original CWD on cleanup.
func testSetup(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .git directory so FindRepoRoot finds this dir.
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Clear environment to prevent interference between tests.
	os.Unsetenv("ADM_AGENT")

	return tmpDir
}

// runCmd executes a cobra command with the given args, capturing stdout.
// Commands write to os.Stdout directly, so we redirect it via a pipe.
func runCmd(args ...string) (string, error) {
	// Redirect stdout to a pipe.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	// Reset all flag values to defaults before each run.
	resetFlags()
	rootCmd.SetArgs(args)

	cmdErr := rootCmd.Execute()

	// Close write end and read all captured output.
	w.Close()
	captured, _ := io.ReadAll(r)
	r.Close()
	os.Stdout = oldStdout

	return string(captured), cmdErr
}

// resetFlags clears persistent flag state between test invocations.
func resetFlags() {
	registerName = ""
	registerTask = ""
	sendFrom = ""
	sendTo = ""
	sendMsg = ""
	broadcastFrom = ""
	broadcastMsg = ""
	claimAgent = ""
	unclaimAgent = ""
	checkClaimFile = ""
	checkClaimAgent = ""
	syncAgent = ""
	syncAckToken = ""
	syncFormat = "json"
	inboxAgent = ""
	useTask = ""
	taskUpdateTask = ""
	purgeDays = 7
	auditLogLimit = 50
}

// ---- Register + Status ----

func TestRegisterAndStatus(t *testing.T) {
	testSetup(t)

	out, err := runCmd("register", "--name", "alice", "--task", "building auth")
	if err != nil {
		t.Fatalf("register: %v\nout: %s", err, out)
	}

	out, err = runCmd("status")
	if err != nil {
		t.Fatalf("status: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("status output missing alice: %s", out)
	}
	if !strings.Contains(out, "online") {
		t.Errorf("status output missing online state: %s", out)
	}
}

func TestRegisterIdempotent(t *testing.T) {
	testSetup(t)

	if _, err := runCmd("register", "--name", "bob", "--task", "task1"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd("register", "--name", "bob", "--task", "task2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd("status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "task2") {
		t.Errorf("expected updated task, got: %s", out)
	}
}

// ---- Send ----

func TestSendHappyPath(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	out, err := runCmd("send", "--from", "alice", "--to", "bob", "--msg", "hello bob")
	if err != nil {
		t.Fatalf("send: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "sent to bob") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestSendUnknownRecipient(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")

	_, err := runCmd("send", "--from", "alice", "--to", "ghost", "--msg", "hello")
	if err == nil {
		t.Fatal("expected error for unknown recipient")
	}
}

func TestSendUnknownSender(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "bob", "--task", "receiver")

	_, err := runCmd("send", "--from", "ghost", "--to", "bob", "--msg", "hello")
	if err == nil {
		t.Fatal("expected error for unknown sender")
	}
}

// ---- Broadcast ----

func TestBroadcastHappyPath(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")
	runCmd("register", "--name", "charlie", "--task", "receiver2")

	out, err := runCmd("broadcast", "--from", "alice", "--msg", "team update")
	if err != nil {
		t.Fatalf("broadcast: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "broadcast to 2 agent(s)") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestBroadcastUnknownSender(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "bob", "--task", "receiver")

	_, err := runCmd("broadcast", "--from", "ghost", "--msg", "hello")
	if err == nil {
		t.Fatal("expected error for unknown sender in broadcast")
	}
}

func TestBroadcastExcludesSender(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")

	out, err := runCmd("broadcast", "--from", "alice", "--msg", "just me")
	if err != nil {
		t.Fatalf("broadcast: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "broadcast to 0 agent(s)") {
		t.Errorf("expected 0 recipients, got: %s", out)
	}
}

// ---- Claim / Unclaim / Check-claim ----

func TestClaimAndCheckClaim(t *testing.T) {
	testSetup(t)

	os.MkdirAll("src/auth", 0o755)
	os.WriteFile("src/auth/login.go", []byte("package auth"), 0o644)

	out, err := runCmd("claim", "--agent", "alice", "src/auth")
	if err != nil {
		t.Fatalf("claim: %v\nout: %s", err, out)
	}

	// Bob checks the claimed file.
	out, err = runCmd("check-claim", "--file", "src/auth/login.go", "--agent", "bob")
	if err != nil {
		t.Fatalf("check-claim: %v\nout: %s", err, out)
	}

	var w claimWarning
	if err := json.Unmarshal([]byte(out), &w); err != nil {
		t.Fatalf("parse check-claim output: %v\nraw: %s", err, out)
	}
	if !w.Claimed {
		t.Error("expected file to be claimed")
	}
	if w.Owner != "alice" {
		t.Errorf("expected owner alice, got %s", w.Owner)
	}
}

func TestClaimIdempotent(t *testing.T) {
	testSetup(t)

	_, err := runCmd("claim", "--agent", "alice", "src/auth")
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	_, err = runCmd("claim", "--agent", "alice", "src/auth")
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
}

func TestCheckClaimNoMatch(t *testing.T) {
	testSetup(t)

	out, err := runCmd("check-claim", "--file", "src/api/routes.go", "--agent", "bob")
	if err != nil {
		t.Fatalf("check-claim: %v\nout: %s", err, out)
	}

	var w claimWarning
	if err := json.Unmarshal([]byte(out), &w); err != nil {
		t.Fatalf("parse: %v\nraw: %s", err, out)
	}
	if w.Claimed {
		t.Error("expected file to not be claimed")
	}
}

func TestCheckClaimOwnFileNotWarned(t *testing.T) {
	testSetup(t)

	runCmd("claim", "--agent", "alice", "src/auth")

	out, err := runCmd("check-claim", "--file", "src/auth/login.go", "--agent", "alice")
	if err != nil {
		t.Fatalf("check-claim: %v\nout: %s", err, out)
	}

	var w claimWarning
	if err := json.Unmarshal([]byte(out), &w); err != nil {
		t.Fatalf("parse: %v\nraw: %s", err, out)
	}
	if w.Claimed {
		t.Error("expected no warning for own claim")
	}
}

func TestUnclaim(t *testing.T) {
	testSetup(t)

	runCmd("claim", "--agent", "alice", "src/auth")

	out, err := runCmd("unclaim", "--agent", "alice", "src/auth")
	if err != nil {
		t.Fatalf("unclaim: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("unexpected output: %s", out)
	}

	// Check-claim should now show unclaimed.
	out, err = runCmd("check-claim", "--file", "src/auth/login.go", "--agent", "bob")
	if err != nil {
		t.Fatal(err)
	}
	var w claimWarning
	json.Unmarshal([]byte(out), &w)
	if w.Claimed {
		t.Error("expected file to be unclaimed after unclaim")
	}
}

func TestUnclaimNonexistent(t *testing.T) {
	testSetup(t)

	out, err := runCmd("unclaim", "--agent", "alice", "src/nothing")
	if err != nil {
		t.Fatalf("unclaim: %v", err)
	}
	if !strings.Contains(out, "no claim found") {
		t.Errorf("unexpected output: %s", out)
	}
}

// ---- Sync ----

func TestSyncEmptyQueue(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "idle")

	out, err := runCmd("sync", "--agent", "alice", "--format", "json")
	if err != nil {
		t.Fatalf("sync: %v\nout: %s", err, out)
	}

	var resp syncResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse sync response: %v\nraw: %s", err, out)
	}
	if len(resp.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(resp.Messages))
	}
	if resp.BatchToken != "" {
		t.Errorf("expected empty batch_token, got %q", resp.BatchToken)
	}
}

func TestSyncDeliveryLifecycle(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "hello bob")

	// First sync: bob gets the message (pending -> offered).
	out, err := runCmd("sync", "--agent", "bob", "--format", "json")
	if err != nil {
		t.Fatalf("sync1: %v\nout: %s", err, out)
	}

	var resp1 syncResponse
	if err := json.Unmarshal([]byte(out), &resp1); err != nil {
		t.Fatalf("parse sync1: %v\nraw: %s", err, out)
	}

	if len(resp1.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp1.Messages))
	}
	if resp1.Messages[0].From != "alice" {
		t.Errorf("expected from=alice, got %s", resp1.Messages[0].From)
	}
	if resp1.Messages[0].Body != "hello bob" {
		t.Errorf("expected body='hello bob', got %s", resp1.Messages[0].Body)
	}
	if resp1.BatchToken == "" {
		t.Fatal("expected non-empty batch_token")
	}

	// Second sync without ack: should get 0 messages (already offered).
	out, err = runCmd("sync", "--agent", "bob", "--format", "json")
	if err != nil {
		t.Fatalf("sync2: %v\nout: %s", err, out)
	}

	var resp2 syncResponse
	json.Unmarshal([]byte(out), &resp2)
	if len(resp2.Messages) != 0 {
		t.Errorf("expected 0 messages (already offered), got %d", len(resp2.Messages))
	}

	// Third sync with ack-token: acknowledges and returns empty.
	out, err = runCmd("sync", "--agent", "bob", "--ack-token", resp1.BatchToken, "--format", "json")
	if err != nil {
		t.Fatalf("sync3: %v\nout: %s", err, out)
	}

	var resp3 syncResponse
	json.Unmarshal([]byte(out), &resp3)
	if len(resp3.Messages) != 0 {
		t.Errorf("expected 0 messages after ack, got %d", len(resp3.Messages))
	}
}

func TestSyncMultipleMessages(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "msg1")
	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "msg2")
	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "msg3")

	out, err := runCmd("sync", "--agent", "bob", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}

	var resp syncResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse: %v\nraw: %s", err, out)
	}
	if len(resp.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Body != "msg1" {
		t.Errorf("expected msg1 first, got %s", resp.Messages[0].Body)
	}
	if resp.Messages[2].Body != "msg3" {
		t.Errorf("expected msg3 last, got %s", resp.Messages[2].Body)
	}
}

// ---- Inbox ----

func TestInboxShowsPendingMessages(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "check this")

	out, err := runCmd("inbox", "--agent", "bob")
	if err != nil {
		t.Fatalf("inbox: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("expected pending state in inbox: %s", out)
	}
	if !strings.Contains(out, "check this") {
		t.Errorf("expected message body in inbox: %s", out)
	}
}

func TestInboxEmpty(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "idle")

	out, err := runCmd("inbox", "--agent", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No messages.") {
		t.Errorf("expected 'No messages.' got: %s", out)
	}
}

func TestInboxDoesNotMutateState(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")
	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "read-only test")

	// Read inbox twice.
	runCmd("inbox", "--agent", "bob")
	out, err := runCmd("inbox", "--agent", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "read-only test") {
		t.Errorf("expected message still present after second inbox read: %s", out)
	}
}

// ---- Broadcast + Sync integration ----

func TestBroadcastDeliveredViaSyncToAllRecipients(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "worker1")
	runCmd("register", "--name", "charlie", "--task", "worker2")

	runCmd("broadcast", "--from", "alice", "--msg", "all hands")

	// Both bob and charlie should see the broadcast.
	out1, _ := runCmd("sync", "--agent", "bob", "--format", "json")
	out2, _ := runCmd("sync", "--agent", "charlie", "--format", "json")

	var resp1, resp2 syncResponse
	json.Unmarshal([]byte(out1), &resp1)
	json.Unmarshal([]byte(out2), &resp2)

	if len(resp1.Messages) != 1 || resp1.Messages[0].Body != "all hands" {
		t.Errorf("bob didn't get broadcast: %+v", resp1)
	}
	if len(resp2.Messages) != 1 || resp2.Messages[0].Body != "all hands" {
		t.Errorf("charlie didn't get broadcast: %+v", resp2)
	}

	// Alice should NOT get her own broadcast.
	outA, _ := runCmd("sync", "--agent", "alice", "--format", "json")
	var respA syncResponse
	json.Unmarshal([]byte(outA), &respA)
	if len(respA.Messages) != 0 {
		t.Errorf("alice should not receive own broadcast, got %d messages", len(respA.Messages))
	}
}

// ---- FindRepoRoot worktree support ----

func TestFindRepoRootWithGitFile(t *testing.T) {
	// .git as a file (worktree layout) should also be detected.
	tmpDir := t.TempDir()

	// Create .git as a file, not directory.
	gitFile := filepath.Join(tmpDir, ".git")
	os.WriteFile(gitFile, []byte("gitdir: /somewhere/else/.git/worktrees/foo\n"), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// register should work (FindRepoRoot should find .git file).
	_, err := runCmd("register", "--name", "worktree-agent", "--task", "testing worktree")
	if err != nil {
		t.Fatalf("register in worktree: %v", err)
	}
}

// ---- Use (session-based identity) ----

func TestUseCreatesSession(t *testing.T) {
	tmpDir := testSetup(t)

	out, err := runCmd("use", "alice", "--task", "building auth")
	if err != nil {
		t.Fatalf("use: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "active: alice") {
		t.Errorf("expected 'active: alice', got: %s", out)
	}

	// Verify session file was created.
	sessionFile := filepath.Join(tmpDir, ".agents", "adm", "state", "session.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("session file missing agent name: %s", data)
	}

	// Verify agent was registered.
	out, err = runCmd("status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("status should show alice: %s", out)
	}
}

func TestUseWithoutTask(t *testing.T) {
	testSetup(t)

	out, err := runCmd("use", "bob")
	if err != nil {
		t.Fatalf("use: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "active: bob") {
		t.Errorf("expected 'active: bob', got: %s", out)
	}
}

func TestUseThenSendWithoutFrom(t *testing.T) {
	testSetup(t)

	// Set up identity via 'use'.
	runCmd("use", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	// Send without --from; should resolve from session.
	out, err := runCmd("send", "--to", "bob", "--msg", "hello from session")
	if err != nil {
		t.Fatalf("send without --from: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "sent to bob") {
		t.Errorf("unexpected output: %s", out)
	}

	// Verify message arrived.
	out, err = runCmd("sync", "--agent", "bob", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var resp syncResponse
	json.Unmarshal([]byte(out), &resp)
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].From != "alice" {
		t.Errorf("expected from=alice, got %s", resp.Messages[0].From)
	}
}

func TestUseThenBroadcastWithoutFrom(t *testing.T) {
	testSetup(t)

	runCmd("use", "alice", "--task", "sender")
	runCmd("register", "--name", "bob", "--task", "receiver")

	out, err := runCmd("broadcast", "--msg", "team update from session")
	if err != nil {
		t.Fatalf("broadcast without --from: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "broadcast to 1 agent(s)") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestUseThenClaimWithoutAgent(t *testing.T) {
	testSetup(t)

	runCmd("use", "alice", "--task", "owner")

	out, err := runCmd("claim", "src/auth")
	if err != nil {
		t.Fatalf("claim without --agent: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected alice in output: %s", out)
	}
}

func TestUseThenUnclaimWithoutAgent(t *testing.T) {
	testSetup(t)

	runCmd("use", "alice", "--task", "owner")
	runCmd("claim", "src/auth")

	out, err := runCmd("unclaim", "src/auth")
	if err != nil {
		t.Fatalf("unclaim without --agent: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestUseThenSyncWithoutAgent(t *testing.T) {
	testSetup(t)

	runCmd("use", "bob", "--task", "syncing")
	runCmd("register", "--name", "alice", "--task", "sender")
	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "sync test")

	// Sync without --agent; should resolve from session.
	out, err := runCmd("sync", "--format", "json")
	if err != nil {
		t.Fatalf("sync without --agent: %v\nout: %s", err, out)
	}
	var resp syncResponse
	json.Unmarshal([]byte(out), &resp)
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Body != "sync test" {
		t.Errorf("expected 'sync test', got %s", resp.Messages[0].Body)
	}
}

func TestSendFailsWithoutIdentity(t *testing.T) {
	testSetup(t)

	// No 'use', no --from, no ADM_AGENT, no agent file.
	runCmd("register", "--name", "bob", "--task", "receiver")

	_, err := runCmd("send", "--to", "bob", "--msg", "should fail")
	if err == nil {
		t.Fatal("expected error when no identity is available")
	}
	if !strings.Contains(err.Error(), "no agent identity found") {
		t.Errorf("expected identity error, got: %v", err)
	}
}

// ---- Whoami ----

func TestWhoamiWithSession(t *testing.T) {
	testSetup(t)

	runCmd("use", "alice", "--task", "testing")

	out, err := runCmd("whoami")
	if err != nil {
		t.Fatalf("whoami: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected alice, got: %s", out)
	}
}

func TestWhoamiWithEnvVar(t *testing.T) {
	testSetup(t)

	os.Setenv("ADM_AGENT", "env-agent")
	defer os.Unsetenv("ADM_AGENT")

	out, err := runCmd("whoami")
	if err != nil {
		t.Fatalf("whoami: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "env-agent") {
		t.Errorf("expected env-agent, got: %s", out)
	}
}

func TestWhoamiFailsWithoutIdentity(t *testing.T) {
	testSetup(t)

	_, err := runCmd("whoami")
	if err == nil {
		t.Fatal("expected error when no identity is available")
	}
}

// ---- Admin commands ----

func TestAdminRequiresEnvVar(t *testing.T) {
	testSetup(t)

	os.Unsetenv("ADM_ADMIN")

	_, err := runCmd("admin", "audit-log")
	if err == nil {
		t.Fatal("expected error without ADM_ADMIN=1")
	}
	if !strings.Contains(err.Error(), "ADM_ADMIN=1") {
		t.Errorf("expected ADM_ADMIN error, got: %v", err)
	}

	_, err = runCmd("admin", "purge-delivered")
	if err == nil {
		t.Fatal("expected error without ADM_ADMIN=1")
	}
}

func TestAdminAuditLogShowsEntries(t *testing.T) {
	testSetup(t)

	// Generate some audit entries via normal operations.
	runCmd("use", "alice", "--task", "testing")
	runCmd("register", "--name", "bob", "--task", "receiver")
	runCmd("send", "--from", "alice", "--to", "bob", "--msg", "audit test")

	os.Setenv("ADM_ADMIN", "1")
	defer os.Unsetenv("ADM_ADMIN")

	out, err := runCmd("admin", "audit-log", "--limit", "10")
	if err != nil {
		t.Fatalf("admin audit-log: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "AUDIT LOG") {
		t.Errorf("expected AUDIT LOG header: %s", out)
	}
	if !strings.Contains(out, "send") {
		t.Errorf("expected send entry in audit log: %s", out)
	}
}

func TestAdminPurgeDelivered(t *testing.T) {
	testSetup(t)

	os.Setenv("ADM_ADMIN", "1")
	defer os.Unsetenv("ADM_ADMIN")

	out, err := runCmd("admin", "purge-delivered", "--days", "0")
	if err != nil {
		t.Fatalf("admin purge-delivered: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "purged") {
		t.Errorf("expected purged output: %s", out)
	}
}

// ---- Task-update ----

func TestTaskUpdateHappyPath(t *testing.T) {
	testSetup(t)

	runCmd("use", "alice", "--task", "initial task")

	out, err := runCmd("task-update", "--task", "new focus area")
	if err != nil {
		t.Fatalf("task-update: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "task updated") {
		t.Errorf("expected 'task updated', got: %s", out)
	}
	if !strings.Contains(out, "new focus area") {
		t.Errorf("expected new task in output: %s", out)
	}

	// Verify status reflects the update.
	out, err = runCmd("status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "new focus area") {
		t.Errorf("status should show updated task: %s", out)
	}
}

func TestTaskUpdateFailsWithoutIdentity(t *testing.T) {
	testSetup(t)

	_, err := runCmd("task-update", "--task", "should fail")
	if err == nil {
		t.Fatal("expected error when no identity is available")
	}
	if !strings.Contains(err.Error(), "no agent identity found") {
		t.Errorf("expected identity error, got: %v", err)
	}
}

func TestTaskUpdateFailsForUnregisteredAgent(t *testing.T) {
	testSetup(t)

	os.Setenv("ADM_AGENT", "ghost")
	defer os.Unsetenv("ADM_AGENT")

	_, err := runCmd("task-update", "--task", "should fail")
	if err == nil {
		t.Fatal("expected error for unregistered agent")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected registration error, got: %v", err)
	}
}

func TestTaskUpdateWithEnvVar(t *testing.T) {
	testSetup(t)

	runCmd("register", "--name", "env-agent", "--task", "old task")

	os.Setenv("ADM_AGENT", "env-agent")
	defer os.Unsetenv("ADM_AGENT")

	out, err := runCmd("task-update", "--task", "updated via env")
	if err != nil {
		t.Fatalf("task-update: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "updated via env") {
		t.Errorf("expected new task in output: %s", out)
	}
}
