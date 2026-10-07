package main

import (
	"bufio"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testForwardSIA       = "0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM]"
	testForwardSIA2      = "0101005B\"SIA-DCS\"0009R0075L0001[#001465|NUR021]"
	testForwardHeartbeat = "SR0001L0001    001465XX    [ID5B9490D8]"
)

// fakeCenter is a monitoring center for tests: it records every frame it
// receives and answers with respond(n, frame), n = 1, 2, ... (no answer if
// respond returns nil: the connection is kept open until the client closes it).
type fakeCenter struct {
	t       *testing.T
	ln      net.Listener
	respond func(n int, frame string) []byte
	frames  chan string
	mu      sync.Mutex
	n       int
}

func newFakeCenter(t *testing.T, addr string, respond func(n int, frame string) []byte) *fakeCenter {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c := &fakeCenter{t: t, ln: ln, respond: respond, frames: make(chan string, 100)}
	go c.serve()
	t.Cleanup(func() { ln.Close() })
	return c
}

func (c *fakeCenter) Addr() string { return c.ln.Addr().String() }

func (c *fakeCenter) serve() {
	for {
		conn, err := c.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			frame, err := bufio.NewReader(conn).ReadString('\r')
			if err != nil {
				return
			}
			c.mu.Lock()
			c.n++
			n := c.n
			c.mu.Unlock()
			c.frames <- frame
			if resp := c.respond(n, frame); resp != nil {
				conn.Write(resp)
			} else {
				io.Copy(io.Discard, conn) // no answer: wait for the client to give up
			}
		}()
	}
}

// next returns the next received frame (fails after a timeout).
func (c *fakeCenter) next() string {
	c.t.Helper()
	select {
	case f := <-c.frames:
		return f
	case <-time.After(5 * time.Second):
		c.t.Fatal("no frame received")
		return ""
	}
}

// none checks that no (further) frame is received for a while.
func (c *fakeCenter) none() {
	c.t.Helper()
	select {
	case f := <-c.frames:
		c.t.Errorf("unexpected frame %q", f)
	case <-time.After(100 * time.Millisecond):
	}
}

func centerResponse(id string) []byte {
	return DC09Frame(`"` + id + `"0007R0075L0001#001465[]`)
}

func always(id string) func(int, string) []byte {
	return func(int, string) []byte { return centerResponse(id) }
}

var testForwardOptions = ForwarderOptions{
	QueueSize:  10,
	Timeout:    time.Second,
	MinBackoff: 10 * time.Millisecond,
	MaxBackoff: 50 * time.Millisecond,
}

func startForwarder(t *testing.T, spec string, opts ForwarderOptions) *Forwarder {
	t.Helper()
	f, err := NewForwarder(spec, opts)
	if err != nil {
		t.Fatal(err)
	}
	f.Start()
	t.Cleanup(f.Stop)
	return f
}

func enqueue(t *testing.T, f *Forwarder, raw string) *Message {
	t.Helper()
	m := ParseMessage([]byte(raw))
	if !f.Enqueue(m) {
		t.Fatalf("Enqueue(%q) = false", raw)
	}
	return m
}

// frameBody checks a received DC-09 frame and returns its body.
func frameBody(t *testing.T, frame string) string {
	t.Helper()
	return checkFrame(t, []byte(frame))
}

func TestForwarderACK(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", always(ResponseACK))
	f := startForwarder(t, c.Addr(), testForwardOptions)

	m := enqueue(t, f, testForwardSIA)
	want, _ := BuildFrame(m, FormatDC09)
	if got := c.next(); got != string(want) {
		t.Errorf("frame %q, want %q", got, want)
	}
	enqueue(t, f, testForwardSIA2)
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0009`) {
		t.Errorf("second frame %q, want sequence 0009", body)
	}
	c.none() // each message delivered once
}

func TestForwarderNAKRetry(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", func(n int, _ string) []byte {
		if n == 1 {
			return centerResponse(ResponseNAK)
		}
		return centerResponse(ResponseACK)
	})
	f := startForwarder(t, c.Addr(), testForwardOptions)

	enqueue(t, f, testForwardSIA)
	first, second := c.next(), c.next()
	if first != second {
		t.Errorf("retry frame %q differs from %q", second, first)
	}
	c.none()
}

func TestForwarderNAKGiveUp(t *testing.T) {
	// the center keeps rejecting the first message, accepts the second one
	c := newFakeCenter(t, "127.0.0.1:0", func(_ int, frame string) []byte {
		if strings.Contains(frame, "NUA021") {
			return centerResponse(ResponseNAK)
		}
		return centerResponse(ResponseACK)
	})
	f := startForwarder(t, c.Addr(), testForwardOptions)

	enqueue(t, f, testForwardSIA)
	enqueue(t, f, testForwardSIA2)
	for i := 0; i < maxForwardNAKs; i++ {
		if got := c.next(); !strings.Contains(got, "NUA021") {
			t.Fatalf("attempt %d: frame %q, want the first message", i+1, got)
		}
	}
	if got := c.next(); !strings.Contains(got, "NUR021") {
		t.Errorf("frame %q, want the second message", got)
	}
	c.none()
}

func TestForwarderRawUnknownNoRetry(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", always(ResponseNAK))
	f := startForwarder(t, "tcp://"+c.Addr()+"?format=raw", testForwardOptions)

	enqueue(t, f, "garbage")
	if got := c.next(); got != "\ngarbage\r" {
		t.Errorf("frame %q, want %q", got, "\ngarbage\r")
	}
	c.none()
}

func TestForwarderTimeoutRetry(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", func(n int, _ string) []byte {
		if n == 1 {
			return nil // no answer: timeout
		}
		return []byte("ACK")
	})
	opts := testForwardOptions
	opts.Timeout = 100 * time.Millisecond
	f := startForwarder(t, c.Addr(), opts)

	enqueue(t, f, testForwardSIA)
	c.next()
	c.next()
	c.none()
}

func TestForwarderCenterDown(t *testing.T) {
	// reserve an address, then close it: the center is down
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	f := startForwarder(t, "tcp://"+addr, testForwardOptions)
	enqueue(t, f, testForwardSIA)
	time.Sleep(150 * time.Millisecond) // several failed attempts

	c := newFakeCenter(t, addr, always(ResponseACK))
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0008`) {
		t.Errorf("frame %q, want sequence 0008", body)
	}
	c.none()
}

func TestForwarderDUH(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", func(n int, _ string) []byte {
		if n == 1 {
			return centerResponse(ResponseDUH)
		}
		return centerResponse(ResponseACK)
	})
	f := startForwarder(t, c.Addr(), testForwardOptions)

	enqueue(t, f, testForwardSIA)
	enqueue(t, f, testForwardSIA2)
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0008`) {
		t.Errorf("first frame %q, want sequence 0008", body)
	}
	// no retry of the rejected message: the next frame is the second message
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0009`) {
		t.Errorf("second frame %q, want sequence 0009", body)
	}
	c.none()
}

func TestForwarderHeartbeatNoRetry(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", func(_ int, frame string) []byte {
		if strings.Contains(frame, `"NULL"`) {
			return centerResponse(ResponseNAK)
		}
		return centerResponse(ResponseACK)
	})
	f := startForwarder(t, c.Addr(), testForwardOptions)

	enqueue(t, f, testForwardHeartbeat)
	enqueue(t, f, testForwardSIA)
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"NULL"`) || !strings.HasSuffix(body, "L0001#001465[]") {
		t.Errorf("first frame %q, want NULL message", body)
	}
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0008`) {
		t.Errorf("second frame %q, want the SIA message (no heartbeat retry)", body)
	}
	c.none()
}

func TestForwarderHeartbeatCenterDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	f := startForwarder(t, addr, testForwardOptions)
	enqueue(t, f, testForwardHeartbeat)
	time.Sleep(100 * time.Millisecond) // the single attempt fails

	c := newFakeCenter(t, addr, always(ResponseACK))
	enqueue(t, f, testForwardSIA)
	if body := frameBody(t, c.next()); !strings.HasPrefix(body, `"SIA-DCS"0008`) {
		t.Errorf("frame %q, want the SIA message (no heartbeat retry)", body)
	}
	c.none()
}

func TestForwarderRaw(t *testing.T) {
	c := newFakeCenter(t, "127.0.0.1:0", func(int, string) []byte { return []byte("\"ACK\"\r") })
	f := startForwarder(t, "tcp://"+c.Addr()+"?format=raw", testForwardOptions)
	if f.Format() != FormatRaw {
		t.Errorf("Format() = %q, want %q", f.Format(), FormatRaw)
	}

	enqueue(t, f, testForwardSIA)
	if got, want := c.next(), "\n"+testForwardSIA+"\r"; got != want {
		t.Errorf("frame %q, want %q", got, want)
	}
	// unknown messages are forwarded verbatim in raw format
	enqueue(t, f, "garbage")
	if got := c.next(); got != "\ngarbage\r" {
		t.Errorf("frame %q, want %q", got, "\ngarbage\r")
	}
	c.none()
}

func TestForwarderEnqueue(t *testing.T) {
	opts := testForwardOptions
	opts.QueueSize = 2
	f, err := NewForwarder("127.0.0.1:9", opts) // not started: nothing is consumed
	if err != nil {
		t.Fatal(err)
	}
	if f.Enqueue(ParseMessage([]byte("garbage"))) {
		t.Error("Enqueue(unknown) in dc09 format = true, want false")
	}
	if f.Enqueue(nil) {
		t.Error("Enqueue(nil) = true, want false")
	}
	for i := 0; i < 2; i++ {
		if !f.Enqueue(ParseMessage([]byte(testForwardSIA))) {
			t.Fatalf("Enqueue #%d = false, want true", i+1)
		}
	}
	if f.Enqueue(ParseMessage([]byte(testForwardSIA))) {
		t.Error("Enqueue to a full queue = true, want false")
	}
	f.Stop()
	f.Stop() // second Stop is harmless
	if f.Enqueue(ParseMessage([]byte(testForwardSIA))) {
		t.Error("Enqueue after Stop = true, want false")
	}
}

func TestForwarderStop(t *testing.T) {
	cases := []struct {
		name    string
		respond func(int, string) []byte
		opts    ForwarderOptions
	}{
		// Stop during the backoff
		{"backoff", always(ResponseNAK), ForwarderOptions{Timeout: time.Second, MinBackoff: time.Hour}},
		// Stop while waiting for the response
		{"response", func(int, string) []byte { return nil }, ForwarderOptions{Timeout: time.Hour}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeCenter(t, "127.0.0.1:0", tc.respond)
			f, err := NewForwarder(c.Addr(), tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			f.Start()
			f.Start() // second Start is harmless
			enqueue(t, f, testForwardSIA)
			c.next()

			done := make(chan struct{})
			go func() {
				f.Stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Stop didn't return")
			}
			c.none()
		})
	}
}

func TestForwarderStopNotStarted(t *testing.T) {
	f, err := NewForwarder("localhost:7000", ForwarderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f.Stop()
}

func TestNewForwarderSpec(t *testing.T) {
	ok := []struct {
		spec, name, format string
	}{
		{"localhost:7000", "localhost:7000", FormatDC09},
		{" 10.0.0.1:7000 ", "10.0.0.1:7000", FormatDC09},
		{"tcp://10.0.0.1:7000", "10.0.0.1:7000", FormatDC09},
		{"tcp://10.0.0.1:7000/", "10.0.0.1:7000", FormatDC09},
		{"tcp://center.example.com:7000?format=dc09", "center.example.com:7000", FormatDC09},
		{"tcp://center.example.com:7000?format=raw", "center.example.com:7000", FormatRaw},
		{"tcp://[::1]:7000?format=raw", "[::1]:7000", FormatRaw},
	}
	for _, c := range ok {
		f, err := NewForwarder(c.spec, ForwarderOptions{})
		if err != nil {
			t.Errorf("NewForwarder(%q): %v", c.spec, err)
			continue
		}
		if f.Name() != c.name || f.Format() != c.format {
			t.Errorf("NewForwarder(%q): name %q, format %q; want %q, %q", c.spec, f.Name(), f.Format(), c.name, c.format)
		}
		if f.opts.QueueSize != DefaultForwardQueue || f.opts.Timeout != DefaultForwardTimeout ||
			f.opts.MinBackoff != DefaultForwardMinBackoff || f.opts.MaxBackoff != DefaultForwardMaxBackoff {
			t.Errorf("NewForwarder(%q): options %+v, want defaults", c.spec, f.opts)
		}
	}

	bad := []string{
		"",
		"localhost",
		":7000",
		"localhost:",
		"localhost:port",
		"localhost:0",
		"localhost:70000",
		"udp://localhost:7000",
		"http://localhost:7000",
		"tcp://localhost",
		"tcp://localhost:7000?format=xml",
		"tcp://localhost:7000?format=",
		"tcp://localhost:7000?mode=raw",
		"tcp://localhost:7000?format=raw&format=dc09",
		"tcp://localhost:7000/path",
		"tcp://user@localhost:7000",
	}
	for _, spec := range bad {
		if f, err := NewForwarder(spec, ForwarderOptions{}); err == nil {
			t.Errorf("NewForwarder(%q) = %q, want error", spec, f.Name())
		}
	}
}
