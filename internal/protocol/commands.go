package protocol

import (
	"bytes"
	"fmt"
)

// Each command below is listed with its class/ID and source in SOURCES.md.
// To add a command: add a method here, a test in commands_test.go, and a row
// in SOURCES.md.

// Firmware is the installed firmware version (00/87).
type Firmware struct{ Major, Minor, Internal byte }

func (f Firmware) String() string { return fmt.Sprintf("%d.%d.%d", f.Major, f.Minor, f.Internal) }

// ChargeState is the 07/84 charging code.
type ChargeState byte

func (s ChargeState) String() string {
	switch s {
	case 0:
		return "not charging"
	case 1:
		return "charging"
	case 4:
		return "fast charging"
	default:
		return fmt.Sprintf("charging state %d", byte(s))
	}
}

// Record is one onboard button assignment (02/8c, 02/0c):
// [profile, button, mode, function, size, data[5]].
type Record struct {
	Profile, Button, Mode, Function, Size byte
	Data                                  [5]byte
}

func (r Record) bytes() []byte {
	return append([]byte{r.Profile, r.Button, r.Mode, r.Function, r.Size}, r.Data[:]...)
}

func parseRecord(payload []byte) (Record, error) {
	if len(payload) < 10 {
		return Record{}, fmt.Errorf("assignment reply has %d bytes, want 10", len(payload))
	}
	r := Record{Profile: payload[0], Button: payload[1], Mode: payload[2], Function: payload[3], Size: payload[4]}
	copy(r.Data[:], payload[5:10])
	if r.Size > 5 {
		return Record{}, fmt.Errorf("assignment data size %d exceeds 5", r.Size)
	}
	return r, nil
}

// Serial reads the controller serial number (00/82).
func (c *Client) Serial() (string, error) {
	payload, err := c.Do(0x00, 0x82, make([]byte, 22))
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(payload, "\x00")), nil
}

// Firmware reads the installed firmware version (00/87).
func (c *Client) Firmware() (Firmware, error) {
	payload, err := c.Do(0x00, 0x87, make([]byte, 4))
	if err != nil {
		return Firmware{}, err
	}
	if len(payload) < 4 {
		return Firmware{}, fmt.Errorf("firmware reply has %d bytes, want 4", len(payload))
	}
	return Firmware{payload[0], payload[1], payload[2]}, nil
}

// ActiveProfile reads the active onboard profile (05/84).
func (c *Client) ActiveProfile() (byte, error) {
	payload, err := c.Do(0x05, 0x84, []byte{0})
	if err != nil {
		return 0, err
	}
	if len(payload) != 1 {
		return 0, fmt.Errorf("active-profile reply has %d bytes, want 1", len(payload))
	}
	return payload[0], nil
}

// PollingRate reads a profile's configured polling rate in Hz (00/c0).
func (c *Client) PollingRate(profile byte) (int, error) {
	payload, err := c.Do(0x00, 0xc0, []byte{profile, 0})
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 || payload[0] != profile {
		return 0, fmt.Errorf("polling reply % x does not match profile %d", payload, profile)
	}
	return RateForPeriod(payload[1])
}

// SetPollingRate stores a profile's polling rate (00/40).
func (c *Client) SetPollingRate(profile byte, rate int) error {
	period, err := PeriodForRate(rate)
	if err != nil {
		return err
	}
	_, err = c.Do(0x00, 0x40, []byte{profile, period})
	return err
}

// The polling period is in units of 1/8000 s.
var periods = map[byte]int{1: 8000, 2: 4000, 4: 2000, 8: 1000, 16: 500, 32: 250}

func RateForPeriod(period byte) (int, error) {
	rate, ok := periods[period]
	if !ok {
		return 0, fmt.Errorf("unknown polling period 0x%02x", period)
	}
	return rate, nil
}

func PeriodForRate(rate int) (byte, error) {
	for period, r := range periods {
		if r == rate {
			return period, nil
		}
	}
	return 0, fmt.Errorf("unsupported polling rate %d Hz", rate)
}

// Battery reads the battery level as a percentage (07/80).
func (c *Client) Battery() (int, error) {
	payload, err := c.Do(0x07, 0x80, []byte{0, 0})
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 || payload[0] != 0 {
		return 0, fmt.Errorf("battery reply % x is invalid", payload)
	}
	return int(payload[1]) * 100 / 255, nil
}

// Charging reads the charging state (07/84).
func (c *Client) Charging() (ChargeState, error) {
	payload, err := c.Do(0x07, 0x84, []byte{0, 0})
	if err != nil {
		return 0, err
	}
	if len(payload) != 2 || payload[0] != 0 {
		return 0, fmt.Errorf("charging reply % x is invalid", payload)
	}
	return ChargeState(payload[1]), nil
}

// Buttons lists the IDs of the programmable buttons (02/84).
func (c *Client) Buttons() ([]byte, error) {
	payload, err := c.Do(0x02, 0x84, make([]byte, MaxArgs))
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 || int(payload[0]) >= len(payload) {
		return nil, fmt.Errorf("button-list reply % x is truncated", payload)
	}
	return append([]byte(nil), payload[1:1+int(payload[0])]...), nil
}

// Assignment reads one button's assignment in a profile (02/8c).
func (c *Client) Assignment(profile, button byte) (Record, error) {
	request := Record{Profile: profile, Button: button}
	payload, err := c.Do(0x02, 0x8c, request.bytes())
	if err != nil {
		return Record{}, err
	}
	record, err := parseRecord(payload)
	if err != nil {
		return Record{}, err
	}
	if record.Profile != profile || record.Button != button || record.Mode != 0 {
		return Record{}, fmt.Errorf("assignment reply is for profile %d button 0x%02x mode %d", record.Profile, record.Button, record.Mode)
	}
	return record, nil
}

// SetAssignment stores one button's assignment (02/0c).
func (c *Client) SetAssignment(record Record) error {
	_, err := c.Do(0x02, 0x0c, record.bytes())
	return err
}
