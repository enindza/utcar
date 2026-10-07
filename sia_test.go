package main

import (
	"testing"
)

func TestIsHeartbeatNUL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"SR0001L0001    006969XX    [ID00000000]", true},
		{"SR0001L0001\x00\x00\x00\x00001465XX\x00\x00 \x00[ID5B9490D8]", true},
		{"\nSR0001L0001    001465XX    [ID5B9490D8]\r\x00\x00", true},
		// the old regex matched anything ending with NULs + [..]
		{"garbage\x00\x00[ID5B9490D8]", false},
		{"SR0001L0001    001465XX    [ID5B9490D8] trailing", false},
		{"some other text", false},
	}
	for _, tt := range tests {
		if got := IsHeartbeat([]byte(tt.input)); got != tt.want {
			t.Errorf("IsHeartbeat(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseMessageHeartbeat(t *testing.T) {
	m := ParseMessage([]byte("\nSR0001L0002\x00\x00  \x00001465XX\x00   [ID5B9490D8]\r\x00\x00\x00"))
	if m.Kind != KindHeartbeat {
		t.Fatalf("Kind = %q, want heartbeat (%q)", m.Kind, m.ParseError)
	}
	if m.Receiver != "0001" || m.Line != "0002" || m.Account != "001465" {
		t.Errorf("got R=%q L=%q acct=%q", m.Receiver, m.Line, m.Account)
	}
	if m.Raw != "SR0001L0002\x00\x00  \x00001465XX\x00   [ID5B9490D8]" {
		t.Errorf("Raw = %q", m.Raw)
	}
	if m.ParseError != "" || m.Protocol != "" || len(m.Events) != 0 {
		t.Errorf("unexpected fields: %+v", m)
	}
}

func TestParseMessageSIA(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Message
	}{
		{
			name:  "test message",
			input: "01010053\"SIA-DCS\"0007R0075L0001[#001465|NRP000*'DECKERS'NM]7C9677F21948CC12|#001465\x00\x00\x00\x00\x00",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0007", Receiver: "0075", Line: "0001",
				Account: "001465", Blocks: "[#001465|NRP000*'DECKERS'NM]",
			},
		},
		{
			name:  "README message",
			input: "01010053\"SIA-DCS\"0007R0073L0011[#001365|NUA021*'detector hall'NM]7C9677F21948CC12|#001365",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0007", Receiver: "0073", Line: "0011",
				Account: "001365", Blocks: "[#001365|NUA021*'detector hall'NM]",
			},
		},
		{
			name:  "leading LF as in decrypted data",
			input: "\n0101005B\"SIA-DCS\"0012R0075L0001[#001465|Nri1/BA001]\r",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0012", Receiver: "0075", Line: "0001",
				Account: "001465", Blocks: "[#001465|Nri1/BA001]",
			},
		},
		{
			name:  "hex account in header",
			input: "C2A40041\"SIA-DCS\"0002L0#12AB3F[#12AB3F|NCL501]",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0002", Line: "0",
				Account: "12AB3F", Blocks: "[#12AB3F|NCL501]",
			},
		},
		{
			name:  "hex account only in block",
			input: "ABCD0030\"SIA-DCS\"0003R1L1[#A1B2C3|NOP001]",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0003", Receiver: "1", Line: "1",
				Account: "A1B2C3", Blocks: "[#A1B2C3|NOP001]",
			},
		},
		{
			name:  "timestamp",
			input: "9F2B0058\"SIA-DCS\"0009R0075L0001#001465[#001465|NBA1]_12:30:45,10-07-2026",
			want: Message{
				Protocol: "SIA-DCS", Sequence: "0009", Receiver: "0075", Line: "0001",
				Account: "001465", Blocks: "[#001465|NBA1]", Timestamp: "_12:30:45,10-07-2026",
			},
		},
		{
			name:  "encrypted protocol id, multiple blocks, bracket in text",
			input: "00000000\"*SIA-DCS\"0010R0075L0001[#001465|NUA021*'hall [1]'NM][#001465|NUR021]",
			want: Message{
				Protocol: "*SIA-DCS", Sequence: "0010", Receiver: "0075", Line: "0001",
				Account: "001465", Blocks: "[#001465|NUA021*'hall [1]'NM][#001465|NUR021]",
			},
		},
		{
			name:  "NULL message with empty block",
			input: "\n12340020\"NULL\"0001R0L0#001465[]\r",
			want: Message{
				Protocol: "NULL", Sequence: "0001", Receiver: "0", Line: "0",
				Account: "001465", Blocks: "[]",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ParseMessage([]byte(tt.input))
			if m.Kind != KindSIA {
				t.Fatalf("Kind = %q, want sia (%q)", m.Kind, m.ParseError)
			}
			if m.ParseError != "" {
				t.Errorf("ParseError = %q", m.ParseError)
			}
			if m.Protocol != tt.want.Protocol || m.Sequence != tt.want.Sequence ||
				m.Receiver != tt.want.Receiver || m.Line != tt.want.Line ||
				m.Account != tt.want.Account || m.Blocks != tt.want.Blocks ||
				m.Timestamp != tt.want.Timestamp {
				t.Errorf("got  %+v\nwant %+v", *m, tt.want)
			}
		})
	}
}

func TestParseMessageUnknown(t *testing.T) {
	tests := []string{
		"",
		"some other text",
		"\x00\x00\x00",
		"01010053\"SIA-DCS\"0007R0075L0001[#001465",           // unterminated block
		"01010053\"SIA-DCS\"0007R0075L0001[#001465|NUA*'a]b]", // unterminated text
		"01010053\"SIA-DCS\"0007R0075L0001",                   // no data block
		"01010053\"SIA-DCS\"R0075L0001[#001465|NBA1]",         // no sequence
		"garbage\x00\x00[ID5B9490D8]",
	}
	for _, input := range tests {
		m := ParseMessage([]byte(input))
		if m == nil {
			t.Fatalf("ParseMessage(%q) returned nil", input)
		}
		if m.Kind != KindUnknown {
			t.Errorf("ParseMessage(%q).Kind = %q, want unknown", input, m.Kind)
		}
		if m.ParseError == "" {
			t.Errorf("ParseMessage(%q).ParseError is empty", input)
		}
	}
	m := ParseMessage([]byte("\n  garbage \r\x00"))
	if m.Raw != "  garbage " {
		t.Errorf("Raw = %q", m.Raw)
	}
}
