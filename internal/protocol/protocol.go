// Package protocol implements Razer's 90-byte feature-report command protocol
// as used by Synapse's rzDevice25SagePC controllers (see SOURCES.md).
//
// Report layout, after the report ID byte:
//
//	[0] status  [1] transaction ID  [2..4] reserved  [5] argument size
//	[6] command class  [7] command ID  [8..87] arguments  [88] checksum  [89] reserved
//
// The checksum is the XOR of bytes 2..87.
package protocol

import (
	"errors"
	"fmt"
	"time"
)

const (
	ReportID = 0x0a
	// ReportLen is the full feature report including the report ID byte.
	ReportLen = 91
	MaxArgs   = 80
)

// Status is the first byte of a reply.
type Status byte

const (
	StatusNew         Status = 0x00
	StatusBusy        Status = 0x01
	StatusOK          Status = 0x02
	StatusFailed      Status = 0x03
	StatusNoResponse  Status = 0x04
	StatusUnsupported Status = 0x05
)

// StatusError is a reply whose status is neither pending nor OK.
type StatusError struct {
	Class, ID byte
	Status    Status
}

func (e *StatusError) Error() string {
	command := fmt.Sprintf("command %02x/%02x", e.Class, e.ID)
	switch e.Status {
	case StatusNoResponse:
		return command + ": controller did not respond; it may be asleep or off, press a button and retry"
	case StatusFailed:
		return command + ": controller reported failure"
	case StatusUnsupported:
		return command + ": not supported by this controller"
	default:
		return fmt.Sprintf("%s: unexpected status 0x%02x", command, byte(e.Status))
	}
}

// ErrNoReply means no matching reply arrived before the timeout.
var ErrNoReply = errors.New("no reply from controller before timeout")

// Transport moves feature reports. *hidraw.Device implements it.
type Transport interface {
	SetFeature(report []byte) error
	GetFeature(report []byte) error
}

// Client sends commands over a Transport, one at a time.
type Client struct {
	transport   Transport
	transaction byte
	Timeout     time.Duration
	// Gap is the pause before every transfer (request or reply read). Synapse
	// waits 30 ms between transfers; the V3 Pro 8K receiver stops answering
	// for about a second when a write follows another write too closely.
	Gap time.Duration
}

func NewClient(transport Transport) *Client {
	return &Client{transport: transport, Timeout: 750 * time.Millisecond, Gap: 30 * time.Millisecond}
}

// Encode builds a request report. The argument size byte is len(args).
func Encode(transaction, class, id byte, args []byte) ([ReportLen]byte, error) {
	var report [ReportLen]byte
	if len(args) > MaxArgs {
		return report, fmt.Errorf("command %02x/%02x: %d argument bytes exceed %d", class, id, len(args), MaxArgs)
	}
	report[0] = ReportID
	report[2] = transaction
	report[6] = byte(len(args))
	report[7] = class
	report[8] = id
	copy(report[9:], args)
	report[89] = checksum(&report)
	return report, nil
}

// checksum XORs data bytes 2..87, which are report indexes 3..88.
func checksum(report *[ReportLen]byte) byte {
	var sum byte
	for _, b := range report[3:89] {
		sum ^= b
	}
	return sum
}

// Do sends one command and returns the reply's argument bytes. It never
// resends a request: a missing reply is reported, not retried.
func (c *Client) Do(class, id byte, args []byte) ([]byte, error) {
	c.transaction = c.transaction%31 + 1
	request, err := Encode(c.transaction, class, id, args)
	if err != nil {
		return nil, err
	}
	time.Sleep(c.Gap)
	if err := c.transport.SetFeature(request[:]); err != nil {
		return nil, fmt.Errorf("sending command %02x/%02x: %w", class, id, err)
	}
	deadline := time.Now().Add(c.Timeout)
	for {
		time.Sleep(c.Gap)
		var reply [ReportLen]byte
		reply[0] = ReportID
		if err := c.transport.GetFeature(reply[:]); err != nil {
			return nil, fmt.Errorf("reading reply to %02x/%02x: %w", class, id, err)
		}
		payload, done, err := match(&reply, c.transaction, class, id)
		if done || err != nil {
			return payload, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("command %02x/%02x: %w", class, id, ErrNoReply)
		}
	}
}

// match reports whether reply completes the given request. A stale reply from
// an earlier command or a pending status means keep polling.
func match(reply *[ReportLen]byte, transaction, class, id byte) ([]byte, bool, error) {
	if reply[0] != ReportID || reply[2] != transaction || reply[7] != class || reply[8] != id {
		return nil, false, nil
	}
	switch Status(reply[1]) {
	case StatusNew, StatusBusy:
		return nil, false, nil
	case StatusOK:
	default:
		return nil, true, &StatusError{Class: class, ID: id, Status: Status(reply[1])}
	}
	size := int(reply[6])
	if size > MaxArgs {
		return nil, true, fmt.Errorf("command %02x/%02x: reply size %d exceeds %d", class, id, size, MaxArgs)
	}
	if checksum(reply) != reply[89] {
		return nil, true, fmt.Errorf("command %02x/%02x: reply checksum mismatch", class, id)
	}
	payload := make([]byte, size)
	copy(payload, reply[9:9+size])
	return payload, true, nil
}
