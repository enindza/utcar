package main

import (
	"bytes"
	"regexp"
	"strings"
	"time"
)

// MessageKind classifies a decrypted message received from the alarm system.
type MessageKind string

const (
	KindHeartbeat MessageKind = "heartbeat"
	KindSIA       MessageKind = "sia"
	KindUnknown   MessageKind = "unknown"
)

// Event is a single SIA event found in the data block(s) of a message.
type Event struct {
	Code        string // two letters, e.g. "BA"
	Description string // from the SIA code table, "" if unknown
	Zone        string // address/zone as received, e.g. "021", "1", ""
	Area        string // from the ri modifier, "" if absent
	User        string // from the id modifier
	Time        string // from the ti modifier, e.g. "12:30"
	Text        string // text from *'...'
}

// Message is a parsed message received from the alarm system.
type Message struct {
	Time       time.Time // time of reception (set by handleConnection)
	Remote     string    // address of the panel (set by handleConnection)
	Raw        string    // decrypted message, without LF/CR/NUL at the ends
	Kind       MessageKind
	Protocol   string // e.g. "SIA-DCS" (without quotes)
	Sequence   string // "0007"
	Receiver   string // "0075" (without 'R'), "" if absent
	Line       string // "0001" (without 'L'), "" if absent
	Account    string // "001465" (without '#')
	Blocks     string // all data blocks verbatim, with brackets
	Timestamp  string // DC-09 timestamp if present ("_HH:MM:SS,MM-DD-YYYY"), else ""
	Events     []Event
	ParseError string // "" if parsing succeeded
}

var (
	// SR0001L0001    001465XX    [ID5B9490D8]
	// Note: there might be NUL chars instead of (or mixed with) spaces.
	hbRegex = regexp.MustCompile(`^SR(\d{4})L(\d{4})[\s\x00]+(\w+)[\s\x00]+\[\w*\]$`)
	// "SIA-DCS"0007R0075L0001#001465[ ... (CRC and length precede the quote)
	headerRegex = regexp.MustCompile(`"([^"\s]+)"(\d{4})(?:R([0-9A-Fa-f]{1,6}))?(?:L([0-9A-Fa-f]{1,6}))?(?:#([0-9A-Fa-f]{3,16}))?\[`)
	// account in the first data block: [#001465|...
	blockAccountRegex = regexp.MustCompile(`^\[#([0-9A-Fa-f]{3,16})[|\]]`)
	timestampRegex    = regexp.MustCompile(`_\d{2}:\d{2}:\d{2},\d{2}-\d{2}-\d{4}`)
)

func IsHeartbeat(input []byte) bool {
	return hbRegex.Match(bytes.Trim(input, "\n\r\x00"))
}

// ParseMessage parses a decrypted message. It never returns nil and never
// panics; a message that can't be recognized has Kind KindUnknown and a
// non-empty ParseError.
func ParseMessage(data []byte) *Message {
	data = bytes.Trim(data, "\n\r\x00")
	m := &Message{Raw: string(data), Kind: KindUnknown}

	if match := hbRegex.FindSubmatch(data); match != nil {
		m.Kind = KindHeartbeat
		m.Receiver = string(match[1])
		m.Line = string(match[2])
		m.Account = strings.TrimRight(string(match[3]), "X")
		return m
	}

	loc := headerRegex.FindStringSubmatchIndex(m.Raw)
	if loc == nil {
		m.ParseError = "no heartbeat or SIA header found"
		return m
	}
	sub := func(i int) string {
		if loc[2*i] < 0 {
			return ""
		}
		return m.Raw[loc[2*i]:loc[2*i+1]]
	}
	m.Protocol = sub(1)
	m.Sequence = sub(2)
	m.Receiver = sub(3)
	m.Line = sub(4)
	m.Account = sub(5)

	start := loc[1] - 1 // position of the first '['
	end, ok := scanBlocks(m.Raw, start)
	if !ok {
		m.ParseError = "unterminated data block"
		return m
	}
	m.Blocks = m.Raw[start:end]
	if m.Account == "" {
		if match := blockAccountRegex.FindStringSubmatch(m.Blocks); match != nil {
			m.Account = match[1]
		}
	}
	m.Timestamp = timestampRegex.FindString(m.Raw[end:])
	m.Kind = KindSIA
	return m
}

// scanBlocks scans consecutive [...] groups starting at s[start] == '['.
// A ']' inside '...' text doesn't close a block. It returns the position
// right after the last closing bracket.
func scanBlocks(s string, start int) (int, bool) {
	i := start
	for i < len(s) && s[i] == '[' {
		inText := false
		closed := false
		for i++; i < len(s); i++ {
			c := s[i]
			if c == '\'' {
				inText = !inText
			} else if c == ']' && !inText {
				closed = true
				i++
				break
			}
		}
		if !closed {
			return i, false
		}
	}
	return i, true
}
