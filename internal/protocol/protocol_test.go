package protocol

import (
	"errors"
	"testing"
	"time"
)

// fake answers each request with a reply built by respond.
type fake struct {
	request [ReportLen]byte
	respond func(request [ReportLen]byte) [ReportLen]byte
}

func (f *fake) SetFeature(report []byte) error { copy(f.request[:], report); return nil }
func (f *fake) GetFeature(report []byte) error {
	reply := f.respond(f.request)
	copy(report, reply[:])
	return nil
}

// reply echoes the request header with a status and payload, like the
// controller does.
func reply(request [ReportLen]byte, status Status, payload []byte) [ReportLen]byte {
	r, _ := Encode(request[2], request[7], request[8], payload)
	r[1] = byte(status)
	r[89] = checksum(&r)
	return r
}

func client(respond func([ReportLen]byte) [ReportLen]byte) *Client {
	c := NewClient(&fake{respond: respond})
	c.Gap = 0
	c.Timeout = 50 * time.Millisecond
	return c
}

func TestEncodeMatchesCapturedRequest(t *testing.T) {
	// Get Serial Number as logged by Synapse: size 22, class 00, ID 82.
	r, err := Encode(0x1f, 0x00, 0x82, make([]byte, 22))
	if err != nil {
		t.Fatal(err)
	}
	if r[0] != ReportID || r[2] != 0x1f || r[6] != 22 || r[7] != 0x00 || r[8] != 0x82 {
		t.Fatalf("header % x", r[:9])
	}
	if r[89] != 22^0x82^0x00 {
		t.Fatalf("checksum 0x%02x", r[89])
	}
}

func TestEncodeRejectsOversizedArguments(t *testing.T) {
	if _, err := Encode(1, 0, 0, make([]byte, MaxArgs+1)); err == nil {
		t.Fatal("expected error")
	}
}

func TestDoReturnsPayload(t *testing.T) {
	c := client(func(req [ReportLen]byte) [ReportLen]byte { return reply(req, StatusOK, []byte{0x01, 0x04}) })
	rate, err := c.PollingRate(1)
	if err != nil || rate != 2000 {
		t.Fatalf("rate %d, err %v", rate, err)
	}
}

func TestDoWaitsThroughBusyAndStaleReplies(t *testing.T) {
	calls := 0
	c := client(func(req [ReportLen]byte) [ReportLen]byte {
		calls++
		switch calls {
		case 1:
			stale := req
			stale[2]-- // reply to an earlier transaction
			return reply(stale, StatusOK, []byte{9})
		case 2:
			return reply(req, StatusBusy, nil)
		}
		return reply(req, StatusOK, []byte{1})
	})
	profile, err := c.ActiveProfile()
	if err != nil || profile != 1 || calls != 3 {
		t.Fatalf("profile %d, err %v, calls %d", profile, err, calls)
	}
}

func TestNoResponseStatusIsReported(t *testing.T) {
	c := client(func(req [ReportLen]byte) [ReportLen]byte { return reply(req, StatusNoResponse, nil) })
	_, err := c.ActiveProfile()
	var status *StatusError
	if !errors.As(err, &status) || status.Status != StatusNoResponse {
		t.Fatalf("err %v", err)
	}
}

func TestCorruptChecksumIsRejected(t *testing.T) {
	c := client(func(req [ReportLen]byte) [ReportLen]byte {
		r := reply(req, StatusOK, []byte{1})
		r[89] ^= 0xff
		return r
	})
	if _, err := c.ActiveProfile(); err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestMissingReplyTimesOut(t *testing.T) {
	c := client(func(req [ReportLen]byte) [ReportLen]byte { return reply(req, StatusBusy, nil) })
	if _, err := c.ActiveProfile(); !errors.Is(err, ErrNoReply) {
		t.Fatalf("err %v", err)
	}
}

func TestTransactionIDCyclesThrough1To31(t *testing.T) {
	var seen []byte
	c := client(func(req [ReportLen]byte) [ReportLen]byte {
		seen = append(seen, req[2])
		return reply(req, StatusOK, []byte{1})
	})
	for i := 0; i < 32; i++ {
		if _, err := c.ActiveProfile(); err != nil {
			t.Fatal(err)
		}
	}
	if seen[0] != 1 || seen[30] != 31 || seen[31] != 1 {
		t.Fatalf("transactions %v", seen)
	}
}

func TestCapturedReplies(t *testing.T) {
	// Replies recorded from a Wolverine V3 Pro 8K PC on 2026-11-05.
	tests := []struct {
		name    string
		payload []byte
		check   func(c *Client) error
	}{
		{"firmware", []byte{0x01, 0x04, 0x00, 0x00, 0x01, 0x00, 0x07, 0x00}, func(c *Client) error {
			f, err := c.Firmware()
			if err == nil && f.String() != "1.4.0" {
				return errors.New(f.String())
			}
			return err
		}},
		{"buttons", []byte{0x07, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x3d}, func(c *Client) error {
			ids, err := c.Buttons()
			if err == nil && (len(ids) != 7 || ids[6] != 0x3d) {
				return errors.New("wrong ids")
			}
			return err
		}},
		{"battery", []byte{0x00, 0xdf}, func(c *Client) error {
			b, err := c.Battery()
			if err == nil && b != 87 {
				return errors.New("wrong battery")
			}
			return err
		}},
		{"serial", append([]byte("XX0000A00000000"), make([]byte, 7)...), func(c *Client) error {
			s, err := c.Serial()
			if err == nil && s != "XX0000A00000000" {
				return errors.New(s)
			}
			return err
		}},
		{"share assignment", []byte{0x01, 0x3d, 0x00, 0x02, 0x02, 0x0c, 0x46, 0, 0, 0}, func(c *Client) error {
			r, err := c.Assignment(1, 0x3d)
			if err == nil && (r.Function != 2 || r.Data[0] != 0x0c || r.Data[1] != 0x46) {
				return errors.New("wrong record")
			}
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client(func(req [ReportLen]byte) [ReportLen]byte { return reply(req, StatusOK, tt.payload) })
			if err := tt.check(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPollingRateConversion(t *testing.T) {
	for period, rate := range map[byte]int{1: 8000, 2: 4000, 4: 2000, 8: 1000, 16: 500, 32: 250} {
		if got, _ := RateForPeriod(period); got != rate {
			t.Errorf("period %d: %d", period, got)
		}
		if got, _ := PeriodForRate(rate); got != period {
			t.Errorf("rate %d: %d", rate, got)
		}
	}
	if _, err := PeriodForRate(1500); err == nil {
		t.Error("1500 Hz accepted")
	}
}
