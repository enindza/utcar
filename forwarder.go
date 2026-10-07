package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ForwarderOptions configures a Forwarder. Zero values are replaced by the
// defaults below.
type ForwarderOptions struct {
	QueueSize  int           // messages waiting to be sent (default 1000)
	Timeout    time.Duration // connect, send and response timeout (default 10s)
	MinBackoff time.Duration // first retry delay (default 1s)
	MaxBackoff time.Duration // maximum retry delay (default 60s)
}

// Default forwarder options.
const (
	DefaultForwardQueue      = 1000
	DefaultForwardTimeout    = 10 * time.Second
	DefaultForwardMinBackoff = time.Second
	DefaultForwardMaxBackoff = 60 * time.Second
)

// maxForwardNAKs is the number of consecutive NAK responses after which a
// message is dropped: a center that keeps rejecting a frame (e.g. a raw frame
// without a valid DC-09 CRC, or an outdated timestamp) would otherwise block
// the queue forever.
const maxForwardNAKs = 5

// Forwarder sends messages to a monitoring center over TCP, one connection
// per message, and waits for the center's response. Messages are queued and
// sent in order by a single goroutine, so a slow or unreachable center
// doesn't block the reception of messages from the alarm system (nor the
// other centers).
//
// SIA messages are retried (with an exponential backoff) until the center
// accepts them with "ACK", rejects them with "DUH" or answers maxForwardNAKs
// consecutive times with "NAK"; connection errors and timeouts are retried
// without limit. Other messages (a heartbeat as "NULL" link test, an unknown
// message in raw format) get a single attempt.
type Forwarder struct {
	addr   string // host:port
	format string // FormatDC09 or FormatRaw
	opts   ForwarderOptions

	queue  chan forwardItem
	ctx    context.Context // canceled by Stop
	cancel context.CancelFunc
	start  sync.Once
	wg     sync.WaitGroup
}

// forwardItem is a queued message: its frame is built once, so that every
// attempt sends the same frame (with the same sequence number).
type forwardItem struct {
	frame []byte
	retry bool   // retry until delivered (SIA messages only)
	desc  string // for the log
}

// NewForwarder creates a forwarder for spec: "host:port", "tcp://host:port"
// or "tcp://host:port?format=raw" (format "dc09" is the default). The
// forwarder doesn't send anything until Start is called.
func NewForwarder(spec string, opts ForwarderOptions) (*Forwarder, error) {
	addr, format, err := parseForwardSpec(spec)
	if err != nil {
		return nil, err
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = DefaultForwardQueue
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultForwardTimeout
	}
	if opts.MinBackoff <= 0 {
		opts.MinBackoff = DefaultForwardMinBackoff
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = DefaultForwardMaxBackoff
	}
	if opts.MaxBackoff < opts.MinBackoff {
		opts.MaxBackoff = opts.MinBackoff
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Forwarder{
		addr:   addr,
		format: format,
		opts:   opts,
		queue:  make(chan forwardItem, opts.QueueSize),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// parseForwardSpec returns the address and format of a forwarder spec.
func parseForwardSpec(spec string) (addr, format string, err error) {
	format = FormatDC09
	addr = strings.TrimSpace(spec)
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", "", fmt.Errorf("invalid forward address %q (%v)", spec, err)
		}
		if u.Scheme != "tcp" {
			return "", "", fmt.Errorf("invalid forward address %q: unsupported scheme %q (only tcp)", spec, u.Scheme)
		}
		if u.User != nil || (u.Path != "" && u.Path != "/") || u.Fragment != "" {
			return "", "", fmt.Errorf("invalid forward address %q: expected tcp://host:port[?format=dc09|raw]", spec)
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", "", fmt.Errorf("invalid forward address %q (%v)", spec, err)
		}
		for k, v := range q {
			if k != "format" || len(v) != 1 {
				return "", "", fmt.Errorf("invalid forward address %q: unsupported option %q", spec, k)
			}
			format = v[0]
		}
		if format != FormatDC09 && format != FormatRaw {
			return "", "", fmt.Errorf("invalid forward address %q: unknown format %q (dc09 or raw)", spec, format)
		}
		addr = u.Host
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid forward address %q (%v)", spec, err)
	}
	if n, err := strconv.ParseUint(port, 10, 16); host == "" || err != nil || n == 0 {
		return "", "", fmt.Errorf("invalid forward address %q: expected host:port", spec)
	}
	return addr, format, nil
}

// Name returns the address of the monitoring center (host:port).
func (f *Forwarder) Name() string {
	return f.addr
}

// Format returns the format of the messages sent to the center (FormatDC09
// or FormatRaw).
func (f *Forwarder) Format() string {
	return f.format
}

// Start starts sending queued messages. Calling it again has no effect.
func (f *Forwarder) Start() {
	f.start.Do(func() {
		f.wg.Add(1)
		go f.run()
	})
}

// Stop stops the forwarder: a message being sent (or waiting for a retry) is
// abandoned and queued messages are dropped. It waits for the goroutine
// started by Start to finish. Calling it again has no effect.
func (f *Forwarder) Stop() {
	f.cancel()
	f.wg.Wait()
	if n := len(f.queue); n > 0 {
		log.Printf("Forwarder %s: stopped, %d queued message(s) dropped", f.addr, n)
	}
}

// Enqueue queues m to be sent. It returns false (and logs a warning) if m is
// not queued: the queue is full, the forwarder is stopped, or m can't be
// sent in the forwarder's format (e.g. an unknown message in dc09 format).
func (f *Forwarder) Enqueue(m *Message) bool {
	if f.ctx.Err() != nil {
		return false
	}
	frame, err := BuildFrame(m, f.format)
	if err != nil {
		log.Printf("Forwarder %s: WARNING: message not forwarded (%v)", f.addr, err)
		return false
	}
	item := forwardItem{frame: frame, retry: m.Kind == KindSIA, desc: describeForward(m)}
	select {
	case f.queue <- item:
		return true
	default:
		log.Printf("Forwarder %s: WARNING: queue full, %s dropped", f.addr, item.desc)
		return false
	}
}

// describeForward returns a short description of m for the log.
func describeForward(m *Message) string {
	switch m.Kind {
	case KindHeartbeat:
		return "link test (account " + m.Account + ")"
	case KindSIA:
		return fmt.Sprintf("%s message %s (account %s)", m.Protocol, m.Sequence, m.Account)
	}
	return string(m.Kind) + " message"
}

// run sends the queued messages until Stop is called.
func (f *Forwarder) run() {
	defer f.wg.Done()
	for {
		select {
		case <-f.ctx.Done():
			return
		case item := <-f.queue:
			f.deliver(item)
		}
	}
}

// deliver sends item until the center accepts or rejects it (retrying only
// if item.retry is set), or the forwarder is stopped.
func (f *Forwarder) deliver(item forwardItem) {
	backoff := f.opts.MinBackoff
	naks := 0 // consecutive NAK responses
	for attempt := 1; ; attempt++ {
		status, err := f.send(item.frame)
		if f.ctx.Err() != nil {
			return
		}
		switch {
		case err == nil && status == ResponseACK:
			if item.retry {
				log.Printf("Forwarder %s: delivered %s (attempt %d)", f.addr, item.desc, attempt)
			}
			return
		case err == nil && status == ResponseDUH:
			log.Printf("Forwarder %s: WARNING: %s rejected by the center (DUH), giving up", f.addr, item.desc)
			return
		case err == nil && status == ResponseNAK:
			naks++
			if item.retry && naks >= maxForwardNAKs {
				log.Printf("Forwarder %s: WARNING: %s rejected by the center (%d x NAK), giving up", f.addr, item.desc, naks)
				return
			}
			err = fmt.Errorf("response %s", status)
		case err == nil:
			err = fmt.Errorf("response %s", status)
		default:
			naks = 0
		}
		if !item.retry {
			log.Printf("Forwarder %s: %s not delivered (%v)", f.addr, item.desc, err)
			return
		}
		log.Printf("Forwarder %s: %s not delivered (%v), retrying in %s", f.addr, item.desc, err, backoff)
		t := time.NewTimer(backoff)
		select {
		case <-f.ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		backoff = min(backoff*2, f.opts.MaxBackoff)
	}
}

// send makes a single attempt: connect, write frame, read the response (up
// to CR) and parse it.
func (f *Forwarder) send(frame []byte) (string, error) {
	d := net.Dialer{Timeout: f.opts.Timeout}
	conn, err := d.DialContext(f.ctx, "tcp", f.addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	// Stop interrupts a pending write or read.
	stop := context.AfterFunc(f.ctx, func() { conn.Close() })
	defer stop()

	if err := conn.SetDeadline(time.Now().Add(f.opts.Timeout)); err != nil {
		return "", err
	}
	if _, err := conn.Write(frame); err != nil {
		return "", err
	}
	resp, err := bufio.NewReader(conn).ReadBytes('\r')
	if err != nil && !(errors.Is(err, io.EOF) && len(resp) > 0) {
		return "", err
	}
	return ParseResponse(resp)
}
