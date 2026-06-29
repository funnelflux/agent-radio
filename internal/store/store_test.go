package store

import (
	"context"
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
