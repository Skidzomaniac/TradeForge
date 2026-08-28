package bots

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SOH is the FIX field separator.
const SOH = "\x01"

// FIXBot is an optional FIX 4.2 client for contestants exposing a FIX server.
// It is not part of the default HTTP persona rotation; score FIX submissions
// separately.
type FIXBot struct {
	conn         net.Conn
	reader       *bufio.Reader
	targetAddr   string
	senderCompID string
	targetCompID string
	seqNum       atomic.Int64
	mu           sync.Mutex
}

// NewFIXBot constructs a FIX bot (call Connect before use).
func NewFIXBot(targetAddr, sender, target string) *FIXBot {
	return &FIXBot{targetAddr: targetAddr, senderCompID: sender, targetCompID: target}
}

// Connect dials the FIX server and sends a Logon (35=A).
func (b *FIXBot) Connect() error {
	conn, err := net.DialTimeout("tcp", b.targetAddr, 5*time.Second)
	if err != nil {
		return err
	}
	b.conn = conn
	b.reader = bufio.NewReader(conn)
	logon := b.buildFIXMessage("A", "98=0"+SOH+"108=30"+SOH+"141=Y")
	_, err = b.conn.Write([]byte(logon))
	return err
}

// SubmitOrder sends a NewOrderSingle (35=D) and returns the raw response and latency.
func (b *FIXBot) SubmitOrder(orderID, side string, qty, price float64, limit bool) (map[string]string, time.Duration, error) {
	ordType := "1" // market
	body := "11=" + orderID + SOH + "54=" + side + SOH + "38=" + ftoa(qty) + SOH
	if limit {
		ordType = "2"
		body += "44=" + ftoa(price) + SOH
	}
	body += "40=" + ordType + SOH + "21=1" + SOH + "60=" + time.Now().UTC().Format("20060102-15:04:05.000")
	msg := b.buildFIXMessage("D", body)

	start := time.Now()
	b.mu.Lock()
	_, err := b.conn.Write([]byte(msg))
	b.mu.Unlock()
	if err != nil {
		return nil, 0, err
	}
	line, err := b.reader.ReadString(0x01) // crude: read to next SOH-terminated chunk
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	return parseFIXMessage(line), latency, nil
}

// Close terminates the connection.
func (b *FIXBot) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

// buildFIXMessage assembles a complete FIX 4.2 message with body length and checksum.
func (b *FIXBot) buildFIXMessage(msgType, body string) string {
	seq := b.seqNum.Add(1)
	header := "35=" + msgType + SOH +
		"49=" + b.senderCompID + SOH +
		"56=" + b.targetCompID + SOH +
		"34=" + strconv.FormatInt(seq, 10) + SOH +
		"52=" + time.Now().UTC().Format("20060102-15:04:05.000") + SOH
	payload := header + body + SOH
	bodyLen := len(payload)
	withLen := "8=FIX.4.2" + SOH + "9=" + strconv.Itoa(bodyLen) + SOH + payload
	return withLen + "10=" + checksum(withLen) + SOH
}

// checksum computes the 3-digit FIX checksum (sum of bytes mod 256).
func checksum(s string) string {
	var sum int
	for i := 0; i < len(s); i++ {
		sum += int(s[i])
	}
	return fmt.Sprintf("%03d", sum%256)
}

// parseFIXMessage splits a raw FIX message into tag=value pairs.
func parseFIXMessage(raw string) map[string]string {
	out := map[string]string{}
	for _, field := range strings.Split(raw, SOH) {
		if kv := strings.SplitN(field, "=", 2); len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
