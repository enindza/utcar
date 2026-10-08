package main

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "utcar.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

type storedMessage struct {
	ReceivedAt, Remote, Kind, Protocol, Sequence, Receiver, Line, Account, Raw, ParseError string
}

func readMessage(t *testing.T, s *Store, id int64) storedMessage {
	t.Helper()
	var r storedMessage
	var parseError sql.NullString
	err := s.db.QueryRow(`SELECT received_at, remote, kind, protocol, sequence,
		receiver, line, account, raw, parse_error FROM messages WHERE id = ?`, id).Scan(
		&r.ReceivedAt, &r.Remote, &r.Kind, &r.Protocol, &r.Sequence,
		&r.Receiver, &r.Line, &r.Account, &r.Raw, &parseError)
	if err != nil {
		t.Fatalf("read message %d: %v", id, err)
	}
	r.ParseError = parseError.String
	return r
}

func readEvents(t *testing.T, s *Store, id int64) []Event {
	t.Helper()
	rows, err := s.db.Query(`SELECT code, description, zone, area, user_id, event_time, text
		FROM events WHERE message_id = ? ORDER BY id`, id)
	if err != nil {
		t.Fatalf("read events %d: %v", id, err)
	}
	defer rows.Close()
	var evs []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Code, &e.Description, &e.Zone, &e.Area, &e.User, &e.Time, &e.Text); err != nil {
			t.Fatal(err)
		}
		evs = append(evs, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return evs
}

func TestStoreSaveSIA(t *testing.T) {
	s, _ := openTestStore(t)

	raw := "0101005B\"SIA-DCS\"0008R0075L0001[#001465|Nri1/id7/BA001*'hall'NM][#001465|NOP002]_12:30:45,10-07-2026"
	m := ParseMessage([]byte(raw))
	if m.Kind != KindSIA || len(m.Events) != 2 {
		t.Fatalf("bad test message: kind %s, %d events", m.Kind, len(m.Events))
	}
	// Local time with a non-UTC zone: received_at must be stored as UTC.
	m.Time = time.Date(2026, 10, 7, 14, 30, 45, 123456789, time.FixedZone("CEST", 2*3600))
	m.Remote = "192.0.2.10:4567"

	id, err := s.Save(m)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Save returned id %d", id)
	}

	got := readMessage(t, s, id)
	want := storedMessage{
		ReceivedAt: "2026-10-07T12:30:45.123Z",
		Remote:     "192.0.2.10:4567",
		Kind:       "sia",
		Protocol:   "SIA-DCS",
		Sequence:   "0008",
		Receiver:   "0075",
		Line:       "0001",
		Account:    "001465",
		Raw:        m.Raw,
		ParseError: "",
	}
	if got != want {
		t.Errorf("message row:\n got %+v\nwant %+v", got, want)
	}

	evs := readEvents(t, s, id)
	if !reflect.DeepEqual(evs, m.Events) {
		t.Errorf("events:\n got %+v\nwant %+v", evs, m.Events)
	}
	if len(evs) == 2 && (evs[0].Code != "BA" || evs[0].Zone != "001" || evs[0].Area != "1" ||
		evs[0].User != "7" || evs[0].Text != "hall" || evs[0].Description == "" ||
		evs[1].Code != "OP" || evs[1].Zone != "002") {
		t.Errorf("unexpected events: %+v", evs)
	}
}

func TestStoreSaveHeartbeatAndUnknown(t *testing.T) {
	s, _ := openTestStore(t)
	now := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)

	hb := ParseMessage([]byte("SR0001L0001    001465XX    [ID5B9490D8]"))
	hb.Time = now
	hbID, err := s.Save(hb)
	if err != nil {
		t.Fatalf("Save heartbeat: %v", err)
	}

	un := ParseMessage([]byte("garbage"))
	un.Time = now.Add(time.Second)
	unID, err := s.Save(un)
	if err != nil {
		t.Fatalf("Save unknown: %v", err)
	}
	if unID <= hbID {
		t.Errorf("ids not increasing: heartbeat %d, unknown %d", hbID, unID)
	}

	got := readMessage(t, s, hbID)
	if got.Kind != "heartbeat" || got.Account != "001465" || got.ReceivedAt != "2026-10-07T08:00:00.000Z" ||
		got.Raw != hb.Raw || got.ParseError != "" {
		t.Errorf("heartbeat row: %+v", got)
	}
	if evs := readEvents(t, s, hbID); len(evs) != 0 {
		t.Errorf("heartbeat has %d events", len(evs))
	}

	got = readMessage(t, s, unID)
	if got.Kind != "unknown" || got.Raw != "garbage" || got.ParseError == "" ||
		got.ParseError != un.ParseError || got.ReceivedAt != "2026-10-07T08:00:01.000Z" {
		t.Errorf("unknown row: %+v", got)
	}
	if evs := readEvents(t, s, unID); len(evs) != 0 {
		t.Errorf("unknown has %d events", len(evs))
	}
}

func TestStoreSaveNil(t *testing.T) {
	s, _ := openTestStore(t)
	if _, err := s.Save(nil); err == nil {
		t.Error("Save(nil) returned no error")
	}
}

func TestStoreReopen(t *testing.T) {
	s, path := openTestStore(t)
	m := ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021]"))
	m.Time = time.Now()
	id, err := s.Save(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Opening an existing database must keep the data (CREATE ... IF NOT EXISTS).
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if got := readMessage(t, s2, id); got.Account != "001465" {
		t.Errorf("after reopen: %+v", got)
	}
	if evs := readEvents(t, s2, id); len(evs) != 1 || evs[0].Code != "UA" {
		t.Errorf("events after reopen: %+v", evs)
	}
	id2, err := s2.Save(m)
	if err != nil || id2 <= id {
		t.Errorf("Save after reopen: id %d, err %v", id2, err)
	}

	var mode string
	if err := s2.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, %v; want wal", mode, err)
	}
}

func TestOpenStoreError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "utcar.db")
	if s, err := OpenStore(path); err == nil {
		s.Close()
		t.Error("OpenStore in a missing directory returned no error")
	}
}
