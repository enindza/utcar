package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProcessNil(t *testing.T) {
	var p *Processor
	p.Process(nil)
	for _, raw := range []string{
		"SR0001L0001    001465XX    [ID5B9490D8]",
		"garbage",
		"01010053\"SIA-DCS\"0007R0075L0001[#001465|NRP000*'DECKERS'NM]",
	} {
		p.Process(ParseMessage([]byte(raw)))
	}
	(&Processor{}).Process(ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021]")))
}

func TestProcessPush(t *testing.T) {
	p := &Processor{Push: make(chan SIA, 10)}
	m := ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|Nri1/BA001/BA2][#001465|NOP001]"))
	m.Time = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	p.Process(m)

	want := []SIA{
		{m.Time, "0008", "0075", "0001", "001465", "BA", "001"},
		{m.Time, "0008", "0075", "0001", "001465", "BA", "2"},
		{m.Time, "0008", "0075", "0001", "001465", "OP", "001"},
	}
	if len(p.Push) != len(want) {
		t.Fatalf("pushed %d SIA messages, want %d", len(p.Push), len(want))
	}
	for i, w := range want {
		if got := <-p.Push; got != w {
			t.Errorf("SIA %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestProcessRequests(t *testing.T) {
	var p *Processor
	before := requests.Value()
	p.Process(ParseMessage([]byte("SR0001L0001    001465XX    [ID5B9490D8]")))
	p.Process(ParseMessage([]byte("garbage")))
	if got := requests.Value(); got != before {
		t.Errorf("requests changed from %d to %d for heartbeat/unknown", before, got)
	}
	p.Process(ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021]")))
	if got := requests.Value(); got != before+1 {
		t.Errorf("requests = %d, want %d", got, before+1)
	}
}

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		e    Event
		want string
	}{
		{Event{Code: "BA", Description: "Burglary Alarm", Zone: "001", Area: "1"},
			"BA (Burglary Alarm) zone 001 area 1"},
		{Event{Code: "QQ"}, "QQ (unknown code)"},
		{Event{Code: "OP", Description: "Opening Report", Zone: "1", User: "7", Time: "12:30", Text: "hall"},
			"OP (Opening Report) zone 1 user 7 time 12:30 text 'hall'"},
	}
	for _, tt := range tests {
		if got := formatEvent(tt.e); got != tt.want {
			t.Errorf("formatEvent(%+v) = %q, want %q", tt.e, got, tt.want)
		}
	}
}

// countRows returns the number of rows in table.
func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestProcessStore(t *testing.T) {
	heartbeat := "SR0001L0001    001465XX    [ID5B9490D8]"
	sia := "0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM][#001465|NUR022]"
	for _, tc := range []struct {
		storeHeartbeats bool
		messages        int
	}{
		{false, 2}, // sia + unknown
		{true, 3},  // + heartbeat
	} {
		s, _ := openTestStore(t)
		p := &Processor{Store: s, StoreHeartbeats: tc.storeHeartbeats}
		for _, raw := range []string{heartbeat, sia, "garbage"} {
			p.Process(ParseMessage([]byte(raw)))
		}
		if n := countRows(t, s, "messages"); n != tc.messages {
			t.Errorf("StoreHeartbeats=%t: %d messages stored, want %d", tc.storeHeartbeats, n, tc.messages)
		}
		if n := countRows(t, s, "events"); n != 2 {
			t.Errorf("StoreHeartbeats=%t: %d events stored, want 2", tc.storeHeartbeats, n)
		}
		var kinds []string
		rows, err := s.db.Query("SELECT kind FROM messages ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatal(err)
			}
			kinds = append(kinds, k)
		}
		rows.Close()
		want := []string{"sia", "unknown"}
		if tc.storeHeartbeats {
			want = append([]string{"heartbeat"}, want...)
		}
		if !reflect.DeepEqual(kinds, want) {
			t.Errorf("StoreHeartbeats=%t: kinds %v, want %v", tc.storeHeartbeats, kinds, want)
		}
	}
}

// A database error must not stop the processing (pushing) of a message.
func TestProcessStoreError(t *testing.T) {
	s, _ := openTestStore(t)
	s.Close()
	p := &Processor{Store: s, Push: make(chan SIA, 10)}
	p.Process(ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021]")))
	if len(p.Push) != 1 {
		t.Errorf("pushed %d SIA messages after a database error, want 1", len(p.Push))
	}
}

// queuedFrames returns the frames queued by a (not started) forwarder.
func queuedFrames(f *Forwarder) []string {
	var frames []string
	for len(f.queue) > 0 {
		frames = append(frames, string((<-f.queue).frame))
	}
	return frames
}

func TestProcessForward(t *testing.T) {
	heartbeat := "SR0001L0001    001465XX    [ID5B9490D8]"
	sia := "0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM][#001465|NUR022]"
	siaBody := `"SIA-DCS"0008R0075L0001#001465[#001465|NUA021*'hall'NM][#001465|NUR022]`
	for _, forwardHeartbeats := range []bool{false, true} {
		dc09, err := NewForwarder("127.0.0.1:1", ForwarderOptions{})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := NewForwarder("tcp://127.0.0.1:2?format=raw", ForwarderOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(dc09.Stop)
		t.Cleanup(raw.Stop)
		p := &Processor{Forwarders: []*Forwarder{dc09, raw}, ForwardHeartbeats: forwardHeartbeats}
		for _, r := range []string{heartbeat, sia, "garbage"} {
			p.Process(ParseMessage([]byte(r)))
		}

		// dc09: the unknown message is not forwarded
		frames := queuedFrames(dc09)
		want := 1
		if forwardHeartbeats {
			want = 2
			if body := checkFrame(t, []byte(frames[0])); !strings.HasPrefix(body, `"NULL"`) || !strings.HasSuffix(body, "R0001L0001#001465[]") {
				t.Errorf("ForwardHeartbeats=%t: dc09 frame %q, want NULL message", forwardHeartbeats, body)
			}
		}
		if len(frames) != want {
			t.Fatalf("ForwardHeartbeats=%t: dc09 forwarder got %d frames %q, want %d", forwardHeartbeats, len(frames), frames, want)
		}
		if body := checkFrame(t, []byte(frames[want-1])); body != siaBody {
			t.Errorf("ForwardHeartbeats=%t: dc09 frame %q, want %q", forwardHeartbeats, body, siaBody)
		}

		// raw: everything is forwarded as received
		wantRaw := []string{"\n" + sia + "\r", "\ngarbage\r"}
		if forwardHeartbeats {
			wantRaw = append([]string{"\n" + heartbeat + "\r"}, wantRaw...)
		}
		if got := queuedFrames(raw); !reflect.DeepEqual(got, wantRaw) {
			t.Errorf("ForwardHeartbeats=%t: raw frames %q, want %q", forwardHeartbeats, got, wantRaw)
		}
	}
}

// Forwarding must not stop the processing (pushing) of a message, even if a
// forwarder can't queue it.
func TestProcessForwardQueueFull(t *testing.T) {
	f, err := NewForwarder("127.0.0.1:1", ForwarderOptions{QueueSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Stop)
	p := &Processor{Forwarders: []*Forwarder{f}, Push: make(chan SIA, 10)}
	for range 3 {
		p.Process(ParseMessage([]byte("0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021]")))
	}
	if len(f.queue) != 1 {
		t.Errorf("%d messages queued, want 1", len(f.queue))
	}
	if len(p.Push) != 3 {
		t.Errorf("pushed %d SIA messages, want 3", len(p.Push))
	}
}
