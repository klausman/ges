package main

import (
	"errors"
	"flag"
	"fmt"
	"hash/adler32"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

const version = "v0.0.1"

var (
	// A possible alternative here would be to only use the base64 charset,
	// possibly confusing the other side as to what is going on.
	alnum      = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/")
	alnumlen   = len(alnum)
	delay      = flag.Duration("d", time.Second*3, "Maximum delay between sending random data to client")
	addr       = flag.String("a", ":2222", "IP addr:port to listen on")
	lineLength = flag.Uint64("l", 1400, "Maxium length of line sent every [delay] seconds")
)

func main() {
	flag.Parse()
	log.Printf("ges %s starting up. Listening on %s, delay <=%s, line length <=%d", version, *addr, *delay, *lineLength)
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept() failed: %s", err)
			return
		}
		// Handle one connection
		go handle(conn)
	}
}

func handle(c net.Conn) {
	var err error
	var wt int64 // Write() total bytes
	defer c.Close()
	start := time.Now()
	connid := makeConnID(c)
	remoteaddr := c.RemoteAddr().String()
	log.Printf("[%s] Connection from %s", connid, c.RemoteAddr())
	readbuf := make([]byte, 128)
	// Read everything the client has to send
	// There should be nothing, but better to be safe.
	err = c.SetDeadline(time.Now().Add(time.Second))
	if err != nil {
		log.Printf("%s] %s could not set deadline for Read(): %s", connid, remoteaddr, err)
		return // Exit Goroutine
	}
	for n := 0; n < len(readbuf); n, err = c.Read(readbuf) {
		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("[%s] %s Read error, closing: %s", connid, remoteaddr, err)
			return // Exit Goroutine
		}
		if n == 0 {
			break
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("[%s] %s Timeout", connid, remoteaddr)
			break
		}
	}
	clear(readbuf) // Discard all client data
	// At this point, the client _should_ keep reading until the SSH banner
	// from the server is complete --- which will never happen --- so we just
	// keep writing random data with random delays until the client gives up.
	for {
		err = c.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			log.Printf("[%s] %s could not set deadline for Write(): %s", connid, remoteaddr, err)
			return
		}
		//nolint:gosec // not a security problem
		data := getRandomData(uint64(float64(*lineLength) * rand.Float64()))
		n, err := c.Write(data)
		wt += int64(n)
		if err != nil || n == 0 {
			log.Printf("[%s] %s Connection closed, wrote %d bytes over %s", connid, remoteaddr, wt, time.Since(start))
			return // Exit Goroutine
		}
		//nolint:gosec // not a security problem
		randsleep := time.Duration(float64(*delay) * rand.Float64())
		// log.Printf("[%s] %s Sent %d bytes, sleeping %s", connid, remoteaddr, n, randsleep)
		time.Sleep(randsleep)
	}
}

func getRandomData(amount uint64) []byte {
	// one random byte, two '=' and a '\n'
	if amount < 4 {
		amount = 4
	}
	buf := make([]byte, amount)
	for i := 0; i < len(buf)-3; i++ {
		//nolint:gosec // not a security problem
		buf[i] = alnum[rand.Intn(alnumlen)]
	}
	buf[amount-3] = '='
	buf[amount-2] = '='
	buf[amount-1] = '\n'
	return buf
}

func makeConnID(c net.Conn) string {
	s := adler32.New()
	fmt.Fprintf(s, "%s", c.RemoteAddr())
	fmt.Fprintf(s, "%d", time.Now().Unix())
	return fmt.Sprintf("%x", s.Sum32())
}
