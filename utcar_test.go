package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

func listen() (net.Listener, string) {
	l, e := net.Listen("tcp", "127.0.0.1:0") // any available address
	if e != nil {
		log.Fatalf("net.Listen tcp :0: %v", e)
	}
	return l, l.Addr().String()
}

// startServer accepts a single connection and handles it with p. The
// returned channel is closed when handleConnection returns.
func startServer(t *testing.T, p *Processor) (string, chan struct{}) {
	l, addr := listen()
	t.Cleanup(func() { l.Close() })
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := l.Accept()
		if e != nil {
			return // listener closed (test finished)
		}
		defer conn.Close()
		handleConnection(conn, p)
	}()
	return addr, done
}

// waitDone waits for handleConnection to return.
func waitDone(t *testing.T, done chan struct{}) {
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConnection didn't return")
	}
}

// padBlock pads data with NULs to a multiple of the 3DES block size.
func padBlock(data []byte) []byte {
	for len(data)%8 != 0 {
		data = append(data, 0)
	}
	return data
}

func readKey(c net.Conn, t *testing.T) []byte {
	buf := make([]byte, 1024) // receive buffer
	n, err := c.Read(buf)
	if err != nil {
		if err != io.EOF {
			log.Fatalf("Key read error: %v", err)
		}
	}
	if n != 24 {
		t.Errorf("Expected key length is 24, was %v", n)
	}
	return Scramble(buf[:n])
}

// sendMessage plays the alarm system side: reads the key, sends the
// encrypted message and checks the ACK.
func sendMessage(t *testing.T, addr string, msg string) {
	client, e := net.Dial("tcp", addr)
	if e != nil {
		t.Fatalf("Dial (%v)", e)
	}
	defer client.Close()
	key := readKey(client, t)
	encrypted := Encrypt3DESECB(padBlock([]byte(msg)), key)
	_, e = client.Write(encrypted)
	if e != nil {
		t.Fatalf("Write encrypted (%v)", e)
	}
	buf := make([]byte, 16) // only need 8
	n, e := client.Read(buf)
	if e != nil {
		t.Fatalf("Failed to read ACK (%v)", e)
	}
	if n != 8 {
		t.Fatalf("Expected 8 bytes, read %d", n)
	}
	ack := Decrypt3DESECB(buf[:8], key)
	valid := []byte("ACK\r")
	valid = append(valid, []byte{0, 0, 0, 0}...)
	if !bytes.Equal(valid, ack) {
		t.Fatalf("ACK messages didn't match, was %v", ack)
	}
}

// taking inspiration from http://golang.org/src/pkg/net/rpc/server_test.go
func TestHandleConnection(t *testing.T) {
	addr, done := startServer(t, nil)
	sendMessage(t, addr, "01010053\"SIA-DCS\"0007R0075L0001[#001465|NRP000*'DECKERS'NM]7C9677F21948CC12|#001465")
	waitDone(t, done)
}

func TestHandleConnectionPush(t *testing.T) {
	p := &Processor{Push: make(chan SIA, 10)}
	addr, done := startServer(t, p)
	before := time.Now()
	sendMessage(t, addr, "\n0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM][#001465|NUR022]7C9677F21948CC12|#001465")
	waitDone(t, done)

	want := []SIA{
		{sequence: "0008", receiver: "0075", line: "0001", account: "001465", command: "UA", zone: "021"},
		{sequence: "0008", receiver: "0075", line: "0001", account: "001465", command: "UR", zone: "022"},
	}
	if len(p.Push) != len(want) {
		t.Fatalf("pushed %d SIA messages, want %d", len(p.Push), len(want))
	}
	for i, w := range want {
		got := <-p.Push
		if got.time.Before(before) {
			t.Errorf("SIA %d: time %v is before the message was sent (%v)", i, got.time, before)
		}
		got.time = time.Time{}
		if got != w {
			t.Errorf("SIA %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestHandleConnectionUnknown(t *testing.T) {
	p := &Processor{Push: make(chan SIA, 10)}
	for _, msg := range []string{
		"this is not a SIA message",
		"01010053\"SIA-DCS\"0007R0075L0001[#001465", // unterminated block
	} {
		addr, done := startServer(t, p)
		sendMessage(t, addr, msg)
		waitDone(t, done)
	}
	if len(p.Push) != 0 {
		t.Errorf("unknown message was pushed (%d)", len(p.Push))
	}
}

func TestHandleConnectionHeartbeat(t *testing.T) {
	p := &Processor{Push: make(chan SIA, 10)}
	addr, done := startServer(t, p)
	sendMessage(t, addr, "SR0001L0001    001465XX    [ID5B9490D8]")
	waitDone(t, done)
	if len(p.Push) != 0 {
		t.Errorf("heartbeat was pushed (%d)", len(p.Push))
	}
}
