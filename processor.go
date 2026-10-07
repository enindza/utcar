package main

import (
	"log"
	"strings"
)

// Processor handles messages after they have been received, decrypted,
// acknowledged and parsed: storing, logging, counting and pushing to openHAB.
// Forwarding (step 9) will be added here.
type Processor struct {
	Store           *Store   // nil = no database
	Push            chan SIA // nil = no openHAB pusher
	StoreHeartbeats bool     // store heartbeats too (they arrive every minute)
}

// Process handles a parsed message. p may be nil (only logging and
// counting is done then).
func (p *Processor) Process(m *Message) {
	if m == nil {
		return
	}
	p.store(m)

	switch m.Kind {
	case KindHeartbeat:
		log.Printf("Heartbeat from %s (account %s)", m.Remote, m.Account)
		return
	case KindUnknown:
		log.Printf("WARNING: unrecognized message from %s (%s): %q", m.Remote, m.ParseError, m.Raw)
		return
	}

	requests.Add(1) // accessible through expvar

	log.Printf("%s message from %s: seq %s, receiver %s, line %s, account %s, %d event(s)",
		m.Protocol, m.Remote, m.Sequence, m.Receiver, m.Line, m.Account, len(m.Events))
	for _, e := range m.Events {
		log.Println("  Event:", formatEvent(e))
	}

	if p == nil || p.Push == nil {
		return
	}
	for _, e := range m.Events {
		p.Push <- SIA{m.Time, m.Sequence, m.Receiver, m.Line, m.Account, e.Code, e.Zone}
	}
}

// store saves m in the database (if any). Errors are only logged, so that a
// database problem doesn't stop the processing of messages.
func (p *Processor) store(m *Message) {
	if p == nil || p.Store == nil {
		return
	}
	if m.Kind == KindHeartbeat && !p.StoreHeartbeats {
		return
	}
	if _, err := p.Store.Save(m); err != nil {
		log.Printf("Database error: failed to store %s message from %s (%v)", m.Kind, m.Remote, err)
	}
}

// formatEvent returns a single-line, human readable description of an event.
func formatEvent(e Event) string {
	var b strings.Builder
	b.WriteString(e.Code)
	if e.Description != "" {
		b.WriteString(" (" + e.Description + ")")
	} else {
		b.WriteString(" (unknown code)")
	}
	if e.Zone != "" {
		b.WriteString(" zone " + e.Zone)
	}
	if e.Area != "" {
		b.WriteString(" area " + e.Area)
	}
	if e.User != "" {
		b.WriteString(" user " + e.User)
	}
	if e.Time != "" {
		b.WriteString(" time " + e.Time)
	}
	if e.Text != "" {
		b.WriteString(" text '" + e.Text + "'")
	}
	return b.String()
}
