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
	if m.Protocol == "SIA-DCS" || m.Protocol == "*SIA-DCS" {
		m.Events = parseEvents(m.Blocks)
	}
	return m
}

// parseEvents extracts SIA events from the data blocks of a SIA-DCS
// message. The first block always holds events ("[#acct|Nri1/BA001]" or
// "[Nri1/BA001]"); further blocks only if they start with an account
// ("[#acct|...]"), the rest are DC-09 extended data blocks ([X..], [H..] ...)
// and are ignored.
func parseEvents(blocks string) []Event {
	var events []Event
	for i, start := 0, 0; start < len(blocks) && blocks[start] == '['; i++ {
		end, ok := blockEnd(blocks, start)
		if !ok {
			break
		}
		content := blocks[start+1 : end-1]
		start = end
		if strings.HasPrefix(content, "#") {
			bar := strings.IndexByte(content, '|')
			if bar < 0 {
				continue // "[#acct]" - no events
			}
			content = content[bar+1:]
		} else if i > 0 {
			continue // extended data block
		}
		events = append(events, parseBlockEvents(content)...)
	}
	return events
}

var (
	modifierRegex = regexp.MustCompile(`^([a-z]{2})([0-9:]*)`)
	eventRegex    = regexp.MustCompile(`^[A-Z]{2}`)
)

// validToken reports whether s starts with a modifier or an event code.
func validToken(s string) bool {
	return modifierRegex.MatchString(s) || eventRegex.MatchString(s)
}

// parseBlockEvents parses the data of one block (without "#acct|"), e.g.
// "Nri01/CL501/ri02/CL501" or "NUA021*'detector hall'NM". Modifiers (ri, id,
// ti, ...) apply to all following events in the same block.
func parseBlockEvents(data string) []Event {
	if (strings.HasPrefix(data, "N") || strings.HasPrefix(data, "O")) && validToken(data[1:]) {
		data = data[1:]
	}
	var events []Event
	var area, user, tm string
	for _, tok := range splitTokens(data) {
		for tok != "" {
			if match := modifierRegex.FindStringSubmatch(tok); match != nil {
				switch match[1] {
				case "ri":
					area = match[2]
				case "id":
					user = match[2]
				case "ti":
					tm = match[2]
				}
				tok = tok[len(match[0]):]
				continue
			}
			if eventRegex.MatchString(tok) {
				ev := Event{Code: tok[:2], Description: DescribeSIA(tok[:2]),
					Area: area, User: user, Time: tm}
				zone := tok[2:]
				if star := strings.Index(zone, "*'"); star >= 0 {
					text := zone[star+2:]
					if q := strings.IndexByte(text, '\''); q >= 0 {
						text = text[:q]
					}
					ev.Text = text
					zone = zone[:star]
				}
				ev.Zone = zone
				events = append(events, ev)
			}
			break // anything else is ignored
		}
	}
	return events
}

// splitTokens splits s on '/' that are not inside '...' text.
func splitTokens(s string) []string {
	var tokens []string
	inText := false
	last := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			inText = !inText
		case '/':
			if !inText {
				tokens = append(tokens, s[last:i])
				last = i + 1
			}
		}
	}
	return append(tokens, s[last:])
}

// scanBlocks scans consecutive [...] groups starting at s[start] == '['.
// It returns the position right after the last closing bracket.
func scanBlocks(s string, start int) (int, bool) {
	i := start
	for i < len(s) && s[i] == '[' {
		end, ok := blockEnd(s, i)
		if !ok {
			return end, false
		}
		i = end
	}
	return i, true
}

// blockEnd returns the position right after the ']' that closes the block
// starting at s[start] == '['. A ']' inside '...' text doesn't close a block.
func blockEnd(s string, start int) (int, bool) {
	inText := false
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\'':
			inText = !inText
		case ']':
			if !inText {
				return i + 1, true
			}
		}
	}
	return len(s), false
}
