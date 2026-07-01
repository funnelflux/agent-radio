package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	KindSend    = "SEND"
	KindAsk     = "ASK"
	KindReply   = "REPLY"
	KindDone    = "DONE"
	KindDecline = "DECLINE"

	SchemaVersion = 1
)

type Message struct {
	ID       int64
	TS       time.Time
	From     string
	To       string
	Kind     string
	Body     string
	ReplyTo  sql.NullInt64
	ThreadID int64
	Status   string
}

type Store struct {
	db *sql.DB
}

func StateDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("AGENT_RADIO_STATE_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); v != "" {
		return filepath.Join(v, "agent-radio"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "agent-radio"), nil
}

func OpenDefault(ctx context.Context) (*Store, string, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "radio.sqlite")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, "", err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, "", err
	}
	if err := f.Close(); err != nil {
		return nil, "", err
	}
	st, err := Open(ctx, path)
	if err != nil {
		return nil, "", err
	}
	return st, path, nil
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	st := &Store{db: db}
	if err := st.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := st.Prune(ctx, ttl()); err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error { return s.db.Close() }

func ttl() time.Duration {
	hours := 168
	if v := strings.TrimSpace(os.Getenv("AGENT_RADIO_TTL_HOURS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	return time.Duration(hours) * time.Hour
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts TEXT NOT NULL,
  sender TEXT NOT NULL,
  recipient TEXT NOT NULL,
  kind TEXT NOT NULL,
  body TEXT NOT NULL,
  reply_to INTEGER,
  thread_id INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open'
);
CREATE TABLE IF NOT EXISTS reads (
  agent TEXT NOT NULL,
  message_id INTEGER NOT NULL,
  read_at TEXT NOT NULL,
  PRIMARY KEY (agent, message_id)
);
CREATE TABLE IF NOT EXISTS inbox_views (
  agent TEXT NOT NULL,
  n INTEGER NOT NULL,
  message_id INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (agent, n)
);
CREATE TABLE IF NOT EXISTS routes (
  agent TEXT NOT NULL,
  message_id INTEGER NOT NULL,
  routed_at TEXT NOT NULL,
  PRIMARY KEY (agent, message_id)
);
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
CREATE INDEX IF NOT EXISTS idx_messages_recipient_id ON messages(recipient, id);
CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id, id);
`)
	return err
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, errors.New("schema version not found")
	}
	return int(version.Int64), nil
}

func (s *Store) Prune(ctx context.Context, keep time.Duration) error {
	cutoff := time.Now().UTC().Add(-keep).Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
DELETE FROM reads WHERE message_id IN (SELECT id FROM messages WHERE ts < ?);
DELETE FROM inbox_views WHERE message_id IN (SELECT id FROM messages WHERE ts < ?);
DELETE FROM messages WHERE ts < ?;
`, cutoff, cutoff, cutoff)
	return err
}

func (s *Store) Insert(ctx context.Context, from, to, kind, body string, replyTo *int64) (Message, error) {
	from, to, kind = strings.TrimSpace(from), strings.TrimSpace(to), strings.ToUpper(strings.TrimSpace(kind))
	if from == "" || to == "" || body == "" {
		return Message{}, errors.New("from, to, and body are required")
	}
	if !validKind(kind) {
		return Message{}, fmt.Errorf("unsupported kind %q", kind)
	}
	now := time.Now().UTC()
	threadID := int64(0)
	status := "open"
	if replyTo != nil {
		parent, err := s.Get(ctx, *replyTo)
		if err != nil {
			return Message{}, err
		}
		threadID = parent.ThreadID
		if threadID == 0 {
			threadID = parent.ID
		}
		switch kind {
		case KindDone:
			status = "done"
		case KindDecline:
			status = "declined"
		default:
			status = "open"
		}
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO messages(ts,sender,recipient,kind,body,reply_to,thread_id,status) VALUES(?,?,?,?,?,?,?,?)`,
		now.Format(time.RFC3339Nano), from, to, kind, body, nullable(replyTo), threadID, status)
	if err != nil {
		return Message{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	if threadID == 0 {
		threadID = id
		if _, err := s.db.ExecContext(ctx, `UPDATE messages SET thread_id=? WHERE id=?`, threadID, id); err != nil {
			return Message{}, err
		}
	}
	if replyTo != nil && (kind == KindDone || kind == KindDecline) {
		_, _ = s.db.ExecContext(ctx, `UPDATE messages SET status=? WHERE id=?`, status, *replyTo)
	}
	return s.Get(ctx, id)
}

func nullable(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func validKind(kind string) bool {
	switch kind {
	case KindSend, KindAsk, KindReply, KindDone, KindDecline:
		return true
	default:
		return false
	}
}

func (s *Store) Get(ctx context.Context, id int64) (Message, error) {
	rows, err := s.query(ctx, `SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status FROM messages WHERE id=?`, id)
	if err != nil {
		return Message{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Message{}, sql.ErrNoRows
	}
	return scan(rows)
}

func (s *Store) Inbox(ctx context.Context, agent string, peek bool) ([]Message, error) {
	rows, err := s.query(ctx, `
SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status
FROM messages
WHERE (recipient=? OR recipient='all') AND sender<>?
  AND NOT EXISTS (SELECT 1 FROM reads WHERE reads.agent=? AND reads.message_id=messages.id)
ORDER BY id`, agent, agent, agent)
	if err != nil {
		return nil, err
	}
	msgs, err := collect(rows)
	if err != nil {
		return nil, err
	}
	if len(msgs) > 0 {
		if err := s.SaveView(ctx, agent, msgs); err != nil {
			return nil, err
		}
	}
	if !peek {
		if err := s.MarkRead(ctx, agent, msgs); err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func (s *Store) UnreadCount(ctx context.Context, agent string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT count(*)
FROM messages
WHERE (recipient=? OR recipient='all') AND sender<>?
  AND NOT EXISTS (SELECT 1 FROM reads WHERE reads.agent=? AND reads.message_id=messages.id)
`, agent, agent, agent).Scan(&count)
	return count, err
}

func (s *Store) LatestForAgent(ctx context.Context, agent string) (Message, bool, error) {
	rows, err := s.query(ctx, `
SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status
FROM messages
WHERE sender=? OR recipient=? OR recipient='all'
ORDER BY id DESC
LIMIT 1`, agent, agent)
	if err != nil {
		return Message{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Message{}, false, nil
	}
	msg, err := scan(rows)
	return msg, true, err
}

func (s *Store) Recent(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.query(ctx, `
SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status
FROM messages
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func (s *Store) RecentForAgent(ctx context.Context, agent string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.query(ctx, `
SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status
FROM messages
WHERE sender=? OR recipient=? OR recipient='all'
ORDER BY id DESC
LIMIT ?`, agent, agent, limit)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func (s *Store) PendingRoutes(ctx context.Context, all bool, agent string) ([]Message, error) {
	query := `
SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status
FROM messages
WHERE status='open'
  AND recipient<>'all'
  AND NOT EXISTS (
    SELECT 1 FROM routes
    WHERE routes.agent=messages.recipient
      AND routes.message_id=messages.id
  )`
	args := []any{}
	if !all {
		query += ` AND (recipient=? OR recipient='all') AND sender<>?`
		args = append(args, agent, agent)
	}
	query += ` ORDER BY id`
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func (s *Store) MarkRouted(ctx context.Context, agent string, msg Message) error {
	if strings.TrimSpace(agent) == "" || agent == "all" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO routes(agent,message_id,routed_at) VALUES(?,?,?)`, agent, msg.ID, now)
	return err
}

func (s *Store) SaveView(ctx context.Context, agent string, msgs []Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM inbox_views WHERE agent=?`, agent); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, msg := range msgs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO inbox_views(agent,n,message_id,created_at) VALUES(?,?,?,?)`, agent, i+1, msg.ID, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ResolveView(ctx context.Context, agent string, n int) (Message, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT message_id FROM inbox_views WHERE agent=? AND n=?`, agent, n).Scan(&id); err != nil {
		return Message{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) MarkRead(ctx context.Context, agent string, msgs []Message) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, msg := range msgs {
		if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO reads(agent,message_id,read_at) VALUES(?,?,?)`, agent, msg.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Since(ctx context.Context, afterID int64, all bool, agent string) ([]Message, error) {
	query := `SELECT id,ts,sender,recipient,kind,body,reply_to,thread_id,status FROM messages WHERE id>?`
	args := []any{afterID}
	if !all {
		query += ` AND (recipient=? OR recipient='all') AND sender<>?`
		args = append(args, agent, agent)
	}
	query += ` ORDER BY id`
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collect(rows)
}

func (s *Store) MaxID(ctx context.Context) (int64, error) {
	var id sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT max(id) FROM messages`).Scan(&id)
	if err != nil || !id.Valid {
		return 0, err
	}
	return id.Int64, nil
}

func (s *Store) query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, q, args...)
}

func collect(rows *sql.Rows) ([]Message, error) {
	defer rows.Close()
	var out []Message
	for rows.Next() {
		msg, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

func scan(rows interface{ Scan(dest ...any) error }) (Message, error) {
	var msg Message
	var ts string
	if err := rows.Scan(&msg.ID, &ts, &msg.From, &msg.To, &msg.Kind, &msg.Body, &msg.ReplyTo, &msg.ThreadID, &msg.Status); err != nil {
		return Message{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Message{}, err
	}
	msg.TS = parsed
	return msg, nil
}
