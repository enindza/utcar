package main

import (
	"bytes"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type SIA struct {
	time     time.Time
	sequence string
	receiver string
	line     string
	account  string
	command  string
	zone     string
}

type Heartbeat struct {
	time time.Time
}

var (
	pchan chan SIA

	requests = expvar.NewInt("requests")
)

var rootCmd = &cobra.Command{
	Use:   "utcar",
	Short: "Utcar provides integration for ATS2000IP alarm system",
	Long: `Utcar provides integration for ATS2000IP alarm system
			and optionally posts to an Openhab home automation
			system.
			Complete documentation is available at 
			https://github.com/tdeckers/utcar`,
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func Execute() {
	rootCmd.PersistentFlags().String("addr", "", "Target addr (e.g. http://openhab.local:8080)")
	rootCmd.PersistentFlags().String("user", "", "Target username")
	rootCmd.PersistentFlags().String("pwd", "", "Target password")
	rootCmd.PersistentFlags().Int("port", 12300, "Listen port number")
	rootCmd.PersistentFlags().Int("debug", 0, "Debug server port number (default: no debug server)")
	rootCmd.PersistentFlags().String("db", "", "SQLite database file (default: no database)")
	rootCmd.PersistentFlags().Bool("db-heartbeats", false, "Store heartbeat messages in the database")
	rootCmd.PersistentFlags().StringSlice("forward", nil, "Monitoring center to forward messages to: host:port, tcp://host:port or tcp://host:port?format=raw (repeatable or comma separated)")
	rootCmd.PersistentFlags().Duration("forward-timeout", DefaultForwardTimeout, "Monitoring center connect and response timeout")
	rootCmd.PersistentFlags().Int("forward-queue", DefaultForwardQueue, "Messages waiting to be forwarded, per monitoring center")
	rootCmd.PersistentFlags().Bool("forward-heartbeats", true, "Forward heartbeats as DC-09 NULL (link test) messages")
	viper.BindPFlag("addr", rootCmd.PersistentFlags().Lookup("addr"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("pwd", rootCmd.PersistentFlags().Lookup("pwd"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("db", rootCmd.PersistentFlags().Lookup("db"))
	viper.BindPFlag("db-heartbeats", rootCmd.PersistentFlags().Lookup("db-heartbeats"))
	viper.BindPFlag("forward", rootCmd.PersistentFlags().Lookup("forward"))
	viper.BindPFlag("forward-timeout", rootCmd.PersistentFlags().Lookup("forward-timeout"))
	viper.BindPFlag("forward-queue", rootCmd.PersistentFlags().Lookup("forward-queue"))
	viper.BindPFlag("forward-heartbeats", rootCmd.PersistentFlags().Lookup("forward-heartbeats"))

	cobra.OnInitialize(initConfig)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("utcar") // uppercased automatically
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	viper.SetDefault("port", 12300)
}

// forwardSpecs returns the monitoring center addresses given by the
// --forward values. A value may hold several addresses separated by commas:
// viper doesn't split UTCAR_FORWARD.
func forwardSpecs(values []string) []string {
	var specs []string
	for _, v := range values {
		for _, spec := range strings.Split(v, ",") {
			if spec = strings.TrimSpace(spec); spec != "" {
				specs = append(specs, spec)
			}
		}
	}
	return specs
}

// newForwarders creates a (not started) forwarder for every spec.
func newForwarders(specs []string, opts ForwarderOptions) ([]*Forwarder, error) {
	var forwarders []*Forwarder
	for _, spec := range specs {
		f, err := NewForwarder(spec, opts)
		if err != nil {
			for _, f := range forwarders {
				f.Stop()
			}
			return nil, err
		}
		forwarders = append(forwarders, f)
	}
	return forwarders, nil
}

// handleConnection handles connections from the alarm system.
// In short, it accepts a connection and sends a new, encrypted key.  Then it
// receives an encrypted message from the alarm system, after which it completes
// with an ACK message.
func handleConnection(c net.Conn, p *Processor) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Message processing panic (%v)\n", r)
			debug.PrintStack()
		}
	}()
	key := GenerateKey()
	scrambled_key := Scramble(key)
	// Send key to alarm system
	n, err := c.Write(scrambled_key)
	if err != nil {
		log.Panic(err)
	}

	buf := make([]byte, 1024) // receive buffer
	n, err = c.Read(buf)
	if err != nil {
		if err != io.EOF {
			log.Panic("Read error: ", err)
		}
	}
	received := time.Now()
	encryptedData := buf[:n]

	data := Decrypt3DESECB(encryptedData, key)
	// Remove leading/trailing new line, line feeds, NUL chars
	data = bytes.Trim(data, "\n\r\x00")
	log.Println("Message: ", string(data[:]))

	ack := []byte("ACK\r")
	ack = append(ack, []byte{0, 0, 0, 0}...)
	encryptedAck := Encrypt3DESECB(ack, key)
	n, err = c.Write(encryptedAck)
	if err != nil {
		log.Panic(err)
	}

	m := ParseMessage(data)
	m.Time = received
	if addr := c.RemoteAddr(); addr != nil {
		m.Remote = addr.String()
	}
	p.Process(m)
}

func receiveSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		os.Exit(0)
	}()
}

func run() {
	// setup response to CTRL-C
	receiveSignal()
	// Listen on TCP port 12300 on all interfaces
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", viper.GetInt("port")))
	if err != nil {
		log.Fatal(err) // exit.. something serious must be wrong.
	}
	log.Printf("Listing on port %d...", viper.GetInt("port"))
	defer l.Close()

	// setup debug server
	if viper.GetInt("debug") != 0 {
		go func() {
			err = http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("debug")), nil)
		}()
		if err != nil {
			log.Printf("Failed to start debug server (%v)\n", err)
		} else {
			log.Printf("Debug server running on port %d\n", viper.GetInt("debug"))
		}
	}

	// setup pusher channel (if addr is provided)
	if viper.GetString("addr") != "" {
		log.Printf("Pushing to %s\n", viper.GetString("addr"))
		pchan = make(chan SIA)
		go func() {
			for {
				sia := <-pchan
				// TODO: handle panics from this function (if any?)
				err := HttpPost(viper.GetString("addr"), viper.GetString("user"), viper.GetString("pwd"), sia)
				if err != nil {
					log.Printf("Push error: %v", err)
				}
			}
		}()
	}

	// open database (if db is provided)
	var store *Store
	if path := viper.GetString("db"); path != "" {
		// own err: the debug server goroutine may still assign the outer one
		s, err := OpenStore(path)
		if err != nil {
			log.Fatalf("Failed to open database %s (%v)", path, err)
		}
		defer s.Close()
		store = s
		log.Printf("Storing messages in %s (heartbeats: %t)\n", path, viper.GetBool("db-heartbeats"))
	}

	// setup forwarding to monitoring centers (if forward is provided)
	opts := ForwarderOptions{
		QueueSize: viper.GetInt("forward-queue"),
		Timeout:   viper.GetDuration("forward-timeout"),
	}
	// own err: the debug server goroutine may still assign the outer one
	forwarders, ferr := newForwarders(forwardSpecs(viper.GetStringSlice("forward")), opts)
	if ferr != nil {
		log.Fatalf("Failed to setup forwarding (%v)", ferr)
	}
	for _, f := range forwarders {
		f.Start()
		defer f.Stop()
		log.Printf("Forwarding to %s (format %s, heartbeats: %t)\n", f.Name(), f.Format(), viper.GetBool("forward-heartbeats"))
	}

	p := &Processor{
		Store:             store,
		Forwarders:        forwarders,
		Push:              pchan,
		StoreHeartbeats:   viper.GetBool("db-heartbeats"),
		ForwardHeartbeats: viper.GetBool("forward-heartbeats"),
	}

	for { // eternally...
		// Wait for a connection
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		// Handle the connection in a new routine
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c net.Conn) {
			defer c.Close()

			handleConnection(c, p)
		}(conn)
	}
}

func main() {
	Execute()
}
