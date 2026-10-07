package main

import (
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
