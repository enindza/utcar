package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCRC16(t *testing.T) {
	cases := []struct {
		in   string
		want uint16
	}{
		{"123456789", 0xBB3D},
		{"", 0x0000},
		{"A", 0x30C0},
	}
	for _, c := range cases {
		if got := CRC16([]byte(c.in)); got != c.want {
			t.Errorf("CRC16(%q) = %04X, want %04X", c.in, got, c.want)
		}
	}
}

func TestDC09Frame(t *testing.T) {
	body := `"SIA-DCS"0007R0075L0001#001465[#001465|NRP000]`
	got := string(DC09Frame(body))
	want := fmt.Sprintf("\n%04X%04X%s\r", CRC16([]byte(body)), len(body), body)
	if got != want {
		t.Errorf("DC09Frame = %q, want %q", got, want)
	}
	if len(body) != 0x2E || !strings.HasPrefix(got[5:], "002E") {
		t.Errorf("DC09Frame length field: %q (body %d bytes)", got[5:9], len(body))
	}
	// the frame checks out with ParseResponse's CRC/length validation
	if _, err := ParseResponse([]byte(fmt.Sprintf("\n%04X%04X%s\r", CRC16([]byte(`"ACK"0007`)), 9, `"ACK"0007`))); err != nil {
		t.Errorf("ParseResponse of a built frame: %v", err)
	}
}

// checkFrame checks the LF/CR, CRC and length of a DC-09 frame and returns
// its body.
func checkFrame(t *testing.T, frame []byte) string {
	t.Helper()
	s := string(frame)
	if len(s) < 10 || s[0] != '\n' || s[len(s)-1] != '\r' {
		t.Fatalf("bad frame %q", s)
	}
	body := s[9 : len(s)-1]
	if want := fmt.Sprintf("%04X%04X", CRC16([]byte(body)), len(body)); s[1:9] != want {
		t.Errorf("frame %q: CRC/length %s, want %s", s, s[1:9], want)
	}
	return body
}

func TestBuildFrameSIA(t *testing.T) {
	cases := []struct {
		msg  string
		body string
	}{
		{
			"01010053\"SIA-DCS\"0007R0073L0011[#001365|NUA021*'detector hall'NM]7C9677F21948CC12|#001365",
			`"SIA-DCS"0007R0073L0011#001365[#001365|NUA021*'detector hall'NM]`,
		},
		{
			"\n0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM][#001465|NUR022]7C9677F21948CC12|#001465",
			`"SIA-DCS"0008R0075L0001#001465[#001465|NUA021*'hall'NM][#001465|NUR022]`,
		},
		{ // account in the header, no receiver, timestamp
			`"SIA-DCS"0042L0#1234[Nri1/BA001][H12:30:45]_12:30:45,10-07-2026`,
			`"SIA-DCS"0042L0#1234[Nri1/BA001][H12:30:45]_12:30:45,10-07-2026`,
		},
		{ // no line
			`"SIA-DCS"0001R1#ABC[#ABC|NOP001]`,
			`"SIA-DCS"0001R1L0#ABC[#ABC|NOP001]`,
		},
		{ // other protocols are rebuilt the same way
			`"ADM-CID"0003R0L0#1234[#1234|1401 01 001]`,
			`"ADM-CID"0003R0L0#1234[#1234|1401 01 001]`,
		},
	}
	for _, c := range cases {
		m := ParseMessage([]byte(c.msg))
		if m.Kind != KindSIA {
			t.Fatalf("%q: kind %s (%s)", c.msg, m.Kind, m.ParseError)
		}
		frame, err := BuildFrame(m, FormatDC09)
		if err != nil {
			t.Errorf("BuildFrame(%q): %v", c.msg, err)
			continue
		}
		if body := checkFrame(t, frame); body != c.body {
			t.Errorf("BuildFrame(%q) body = %q, want %q", c.msg, body, c.body)
		}
	}
}

func TestBuildFrameHeartbeat(t *testing.T) {
	m := ParseMessage([]byte("SR0001L0001    001465XX    [ID5B9490D8]"))
	if m.Kind != KindHeartbeat {
		t.Fatalf("kind %s", m.Kind)
	}
	var seqs []int
	for i := 0; i < 2; i++ {
		frame, err := BuildFrame(m, FormatDC09)
		if err != nil {
			t.Fatal(err)
		}
		body := checkFrame(t, frame)
		var seq int
		if n, _ := fmt.Sscanf(body, `"NULL"%04d`, &seq); n != 1 || seq < 1 || seq > 9999 {
			t.Fatalf("body %q: bad sequence", body)
		}
		if want := fmt.Sprintf(`"NULL"%04dR0001L0001#001465[]`, seq); body != want {
			t.Errorf("body = %q, want %q", body, want)
		}
		seqs = append(seqs, seq)
	}
	if seqs[1] != seqs[0]%9999+1 {
		t.Errorf("sequences %v: not consecutive", seqs)
	}

	// the counter wraps from 9999 to 0001
	old := nullSequence.Load()
	defer nullSequence.Store(old)
	nullSequence.Store(9998)
	for _, want := range []string{`"NULL"9999`, `"NULL"0001`} {
		frame, err := BuildFrame(m, FormatDC09)
		if err != nil {
			t.Fatal(err)
		}
		if body := checkFrame(t, frame); !strings.HasPrefix(body, want) {
			t.Errorf("body = %q, want prefix %q", body, want)
		}
	}
}

func TestBuildFrameErrors(t *testing.T) {
	sia := ParseMessage([]byte(`"SIA-DCS"0007R0075L0001[#001465|NUA021]`))
	cases := []struct {
		name   string
		m      *Message
		format string
	}{
		{"nil", nil, FormatDC09},
		{"unknown", ParseMessage([]byte("garbage")), FormatDC09},
		{"unterminated", ParseMessage([]byte(`"SIA-DCS"0007R0075L0001[#001465|NUA021`)), FormatDC09},
		{"no account", ParseMessage([]byte(`"SIA-DCS"0007R0075L0001[NUA021]`)), FormatDC09},
		{"bad format", sia, "xml"},
		{"empty format", sia, ""},
		{"raw empty", &Message{Kind: KindUnknown}, FormatRaw},
	}
	for _, c := range cases {
		if frame, err := BuildFrame(c.m, c.format); err == nil {
			t.Errorf("%s: no error, frame %q", c.name, frame)
		}
	}
}

func TestBuildFrameRaw(t *testing.T) {
	for _, msg := range []string{
		"\n01010053\"SIA-DCS\"0007R0073L0011[#001365|NUA021*'detector hall'NM]7C9677F21948CC12|#001365\x00\x00",
		"SR0001L0001    001465XX    [ID5B9490D8]",
		"garbage", // raw sends unknown messages too
	} {
		m := ParseMessage([]byte(msg))
		frame, err := BuildFrame(m, FormatRaw)
		if err != nil {
			t.Errorf("BuildFrame(%q, raw): %v", msg, err)
			continue
		}
		if want := "\n" + strings.Trim(msg, "\n\r\x00") + "\r"; string(frame) != want {
			t.Errorf("BuildFrame(%q, raw) = %q, want %q", msg, frame, want)
		}
	}
}

func TestParseResponse(t *testing.T) {
	frame := func(body string) string {
		return string(DC09Frame(body))
	}
	ok := []struct {
		resp string
		want string
	}{
		{frame(`"ACK"0007R0075L0001#001465[]`), ResponseACK},
		{frame(`"NAK"0000R0L0A0[]_12:30:45,10-07-2026`), ResponseNAK},
		{frame(`"DUH"0007R0075L0001#001465[]`), ResponseDUH},
		{frame(`"ACK"0007R0075L0001#001465[]`) + "\x00\x00", ResponseACK},
		{`"ACK"0007L0#1234[]`, ResponseACK}, // without CRC/length
		{"ACK", ResponseACK},
		{"NAK\r", ResponseNAK},
	}
	for _, c := range ok {
		got, err := ParseResponse([]byte(c.resp))
		if err != nil || got != c.want {
			t.Errorf("ParseResponse(%q) = %q, %v; want %q", c.resp, got, err, c.want)
		}
	}

	ack := frame(`"ACK"0007R0075L0001#001465[]`)
	bad := []string{
		"",
		"\n\r",
		"garbage",
		"ack",
		frame(`"OK"0007[]`),
		frame(`"ACK0007[]`),
		frame(`"SIA-DCS"0007R0075L0001#001465[#001465|NUA021]`),
		"\n0000" + ack[5:],                    // bad CRC
		ack[:5] + "0001" + ack[9:],            // bad length
		"\nZZZZ001C" + ack[9:],                // non-hex CRC
		"\n1234" + ack[9:],                    // short prefix
		strings.Replace(ack, "ACK", "NAK", 1), // CRC doesn't match
	}
	for _, resp := range bad {
		if got, err := ParseResponse([]byte(resp)); err == nil {
			t.Errorf("ParseResponse(%q) = %q, want error", resp, got)
		}
	}
}
