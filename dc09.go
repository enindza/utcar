package main

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

// Formats of the messages sent to a monitoring center (see BuildFrame).
const (
	FormatDC09 = "dc09"
	FormatRaw  = "raw"
)

// Statuses returned by ParseResponse.
const (
	ResponseACK = "ACK" // message accepted
	ResponseNAK = "NAK" // message rejected, retry
	ResponseDUH = "DUH" // message not understood/supported, don't retry
)

// nullSequence numbers the "NULL" (link test) messages built from heartbeats,
// which don't carry a sequence number of their own.
var nullSequence atomic.Uint32

// CRC16 returns the CRC-16/ARC (poly 0x8005 reflected, init 0) of data, as
// used by SIA DC-09.
func CRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// DC09Frame wraps body (`"id"seq...[data]`) in a SIA DC-09 frame:
// LF, CRC and length of body (4 hex digits each), body, CR.
func DC09Frame(body string) []byte {
	return []byte(fmt.Sprintf("\n%04X%04X%s\r", CRC16([]byte(body)), len(body), body))
}

// BuildFrame returns the frame to send to a monitoring center for m.
//
// Format "dc09" rebuilds the DC-09 frame from the parsed fields (with a
// correct CRC and length): a SIA message keeps its protocol, sequence, data
// blocks and timestamp; a heartbeat becomes a "NULL" (link test) message; an
// unknown message is an error. Format "raw" sends the decrypted message
// verbatim (LF + Raw + CR).
func BuildFrame(m *Message, format string) ([]byte, error) {
	if m == nil {
		return nil, errors.New("no message")
	}
	switch format {
	case FormatRaw:
		if m.Raw == "" {
			return nil, errors.New("empty message")
		}
		return []byte("\n" + m.Raw + "\r"), nil
	case FormatDC09:
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}

	if m.Kind != KindSIA && m.Kind != KindHeartbeat {
		return nil, fmt.Errorf("%s message can't be sent in %s format", m.Kind, format)
	}
	if m.Account == "" {
		return nil, fmt.Errorf("%s message without account", m.Kind)
	}
	var b strings.Builder
	if m.Kind == KindSIA {
		b.WriteString(`"` + m.Protocol + `"` + m.Sequence)
		writeRouting(&b, m)
		b.WriteString(m.Blocks + m.Timestamp)
	} else {
		seq := (nullSequence.Add(1)-1)%9999 + 1 // 0001-9999
		fmt.Fprintf(&b, `"NULL"%04d`, seq)
		writeRouting(&b, m)
		b.WriteString("[]")
	}
	return DC09Frame(b.String()), nil
}

// writeRouting writes the receiver (if any), line (L0 if absent) and account
// of m: "R0075L0001#001465".
func writeRouting(b *strings.Builder, m *Message) {
	if m.Receiver != "" {
		b.WriteString("R" + m.Receiver)
	}
	line := m.Line
	if line == "" {
		line = "0"
	}
	b.WriteString("L" + line + "#" + m.Account)
}

// ParseResponse parses the response of a monitoring center to a DC-09
// message: `LF crc 0LLL "ACK"seq Rrcvr Lpref #acct[] CR` (likewise "NAK",
// "DUH"). The CRC and length are checked if present; a bare "ACK" (with or
// without quotes) is accepted too. The status is ResponseACK, ResponseNAK or
// ResponseDUH.
func ParseResponse(resp []byte) (status string, err error) {
	resp = bytes.Trim(resp, "\n\r\x00 ")
	s := string(resp)
	if isResponseStatus(s) {
		return s, nil
	}

	q := strings.IndexByte(s, '"')
	if q < 0 {
		return "", fmt.Errorf("invalid response %q", s)
	}
	body := s[q:]
	if prefix := s[:q]; prefix != "" {
		if len(prefix) != 8 {
			return "", fmt.Errorf("invalid response %q: bad CRC/length", s)
		}
		crc, err1 := strconv.ParseUint(prefix[:4], 16, 16)
		length, err2 := strconv.ParseUint(prefix[4:], 16, 16)
		if err1 != nil || err2 != nil {
			return "", fmt.Errorf("invalid response %q: bad CRC/length", s)
		}
		if int(length) != len(body) {
			return "", fmt.Errorf("invalid response %q: length %d, expected %d", s, len(body), length)
		}
		if uint16(crc) != CRC16([]byte(body)) {
			return "", fmt.Errorf("invalid response %q: CRC %04X, expected %04X", s, CRC16([]byte(body)), crc)
		}
	}

	end := strings.IndexByte(body[1:], '"')
	if end < 0 {
		return "", fmt.Errorf("invalid response %q: unterminated id", s)
	}
	status = body[1 : end+1]
	if !isResponseStatus(status) {
		return "", fmt.Errorf("invalid response %q: unknown id %q", s, status)
	}
	return status, nil
}

func isResponseStatus(s string) bool {
	return s == ResponseACK || s == ResponseNAK || s == ResponseDUH
}
