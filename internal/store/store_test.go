package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInsertInboxReplyThreadAndReadState(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/radio.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	msg, err := st.Insert(ctx, "codex-a", "codex-b", KindAsk, "need review", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ThreadID != msg.ID {
		t.Fatalf("thread id = %d, want %d", msg.ThreadID, msg.ID)
	}

	inbox, err := st.Inbox(ctx, "codex-b", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 || inbox[0].ID != msg.ID {
		t.Fatalf("unexpected inbox: %#v", inbox)
	}

	again, err := st.Inbox(ctx, "codex-b", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("read message appeared again: %#v", again)
	}

	parent, err := st.ResolveView(ctx, "codex-b", 1)
	if err != nil {
		t.Fatal(err)
	}
	replyTo := parent.ID
	reply, err := st.Insert(ctx, "codex-b", parent.From, KindReply, "looks good", &replyTo)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ThreadID != msg.ThreadID || reply.ReplyTo.Int64 != msg.ID {
		t.Fatalf("bad reply threading: %#v", reply)
	}
}

func TestBroadcastUnreadAndSenderFiltering(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/radio.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := st.Insert(ctx, "codex-a", "all", KindSend, "heads up", nil); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"codex-b", "claude-c"} {
		msgs, err := st.Inbox(ctx, agent, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 1 {
			t.Fatalf("%s got %d broadcast messages", agent, len(msgs))
		}
	}
	self, err := st.Inbox(ctx, "codex-a", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(self) != 0 {
		t.Fatalf("sender saw own broadcast: %#v", self)
	}
}

func TestPendingRoutesTracksRoutedWithoutMarkingRead(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/radio.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	msg, err := st.Insert(ctx, "codex-a", "codex-b", KindAsk, "need review", nil)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := st.PendingRoutes(ctx, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != msg.ID {
		t.Fatalf("pending routes = %#v, want message %d", pending, msg.ID)
	}
	if err := st.MarkRouted(ctx, "codex-b", msg); err != nil {
		t.Fatal(err)
	}
	pending, err = st.PendingRoutes(ctx, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("routed message still pending: %#v", pending)
	}
	unread, err := st.UnreadCount(ctx, "codex-b")
	if err != nil {
		t.Fatal(err)
	}
	if unread != 1 {
		t.Fatalf("unread after route = %d, want 1", unread)
	}
}

func TestPruneRemovesOldMessages(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/radio.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	msg, err := st.Insert(ctx, "a", "b", KindSend, "old", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE messages SET ts=? WHERE id=?`, time.Now().Add(-48*time.Hour).UTC().Format(time.RFC3339Nano), msg.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.Prune(ctx, time.Hour); err != nil {
		t.Fatal(err)
	}
	msgs, err := st.Inbox(ctx, "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("old message not pruned: %#v", msgs)
	}
}

func TestSchemaVersionIsRecorded(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, t.TempDir()+"/radio.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	version, err := st.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}
}

func TestOpenDefaultCreatesPrivateStateFiles(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "state")
	t.Setenv("AGENT_RADIO_STATE_DIR", dir)

	st, path, err := OpenDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("state dir mode = %o, want 700", got)
	}
	dbInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := dbInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("state db mode = %o, want 600", got)
	}
}

func TestOpenDefaultHardensExistingStateDB(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "radio.sqlite")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_RADIO_STATE_DIR", dir)

	st, gotPath, err := OpenDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	dbInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := dbInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("state db mode = %o, want 600", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("state dir mode = %o, want 700", got)
	}
}
