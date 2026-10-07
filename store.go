package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// storeTimeFormat is the format of messages.received_at (always UTC).
const storeTimeFormat = "2006-01-02T15:04:05.000Z"

const storeSchema = `
CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    received_at TEXT NOT NULL,
    remote      TEXT,
    kind        TEXT NOT NULL,
    protocol    TEXT,
    sequence    TEXT,
    receiver    TEXT,
    line        TEXT,
    account     TEXT,
    raw         TEXT NOT NULL,
    parse_error TEXT
);
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id  INTEGER NOT NULL REFERENCES messages(id),
    code        TEXT NOT NULL,
    description TEXT,
    zone        TEXT,
    area        TEXT,
    user_id     TEXT,
    event_time  TEXT,
    text        TEXT
);
CREATE INDEX IF NOT EXISTS idx_messages_received_at ON messages(received_at);
CREATE INDEX IF NOT EXISTS idx_events_code ON events(code);
CREATE INDEX IF NOT EXISTS idx_events_zone ON events(zone);
`

// Store saves received messages and their events into an SQLite database.
type Store struct {
	db *sql.DB
}

// OpenStore opens (or creates) the SQLite database at path and creates the
// schema if it does not exist yet.
func OpenStore(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	// SQLite allows a single writer; one connection avoids "database is locked".
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(storeSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema in %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Save stores the message and its events in a single transaction and returns
// the id of the new row in the messages table.
func (s *Store) Save(m *Message) (int64, error) {
	if m == nil {
		return 0, fmt.Errorf("nil message")
	}
	t := m.Time
	if t.IsZero() {
		t = time.Now()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // no-op after Commit

	res, err := tx.Exec(`INSERT INTO messages
		(received_at, remote, kind, protocol, sequence, receiver, line, account, raw, parse_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.UTC().Format(storeTimeFormat), m.Remote, string(m.Kind), m.Protocol,
		m.Sequence, m.Receiver, m.Line, m.Account, m.Raw, m.ParseError)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, e := range m.Events {
		_, err := tx.Exec(`INSERT INTO events
			(message_id, code, description, zone, area, user_id, event_time, text)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, e.Code, e.Description, e.Zone, e.Area, e.User, e.Time, e.Text)
		if err != nil {
			return 0, fmt.Errorf("insert event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
