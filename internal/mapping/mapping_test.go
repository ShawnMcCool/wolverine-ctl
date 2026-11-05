package mapping

import (
	"testing"

	"github.com/ShawnMcCool/wolverine-ctl/internal/protocol"
)

var (
	v3Pro8K = Dialect{TargetZeroDisables: true}
	v4Pro   = Dialect{TargetZeroDisables: false}
)

func record(b ...byte) protocol.Record {
	r := protocol.Record{Profile: b[0], Button: b[1], Mode: b[2], Function: b[3], Size: b[4]}
	copy(r.Data[:], b[5:])
	return r
}

func TestDecodeCapturedRecords(t *testing.T) {
	// Records read from a Wolverine V3 Pro 8K PC on 2026-11-05.
	tests := []struct {
		record protocol.Record
		want   string
	}{
		{record(1, 0x10, 0, 16, 1, 0), "off"},
		{record(1, 0x12, 0, 2, 2, 0x00, 0x68), "key f13"},
		{record(1, 0x3d, 0, 2, 2, 0x0c, 0x46), "key alt+super+printscreen"},
		{record(1, 0x12, 0, 16, 1, 32), "button a"},
		{record(1, 0x12, 0, 0, 0), "off"}, // function 0, as written for the V4 Pro
		{record(1, 0x12, 0, 9, 1, 7), "unknown (function 9, data 07)"},
	}
	for _, tt := range tests {
		if got := Decode(tt.record, v3Pro8K).String(); got != tt.want {
			t.Errorf("% x: got %q, want %q", tt.record, got, tt.want)
		}
	}
}

func TestParseEncodesRecords(t *testing.T) {
	tests := []struct {
		words []string
		want  protocol.Record
	}{
		{[]string{"key", "f13"}, record(1, 0x12, 0, 2, 2, 0x00, 0x68)},
		{[]string{"key", "Alt+Super+PrintScreen"}, record(1, 0x12, 0, 2, 2, 0x0c, 0x46)},
		{[]string{"key", "ctrl+shift+s"}, record(1, 0x12, 0, 2, 2, 0x03, 0x16)},
		{[]string{"key", "ctrl"}, record(1, 0x12, 0, 2, 2, 0x00, 0xe0)},
		{[]string{"key", "rsuper"}, record(1, 0x12, 0, 2, 2, 0x00, 0xe7)},
		{[]string{"button", "A"}, record(1, 0x12, 0, 16, 1, 32)},
		{[]string{"button", "menu"}, record(1, 0x12, 0, 16, 1, 53)},
		{[]string{"off"}, record(1, 0x12, 0, 16, 1, 0)},
	}
	for _, tt := range tests {
		a, err := Parse(tt.words, v3Pro8K)
		if err != nil {
			t.Errorf("%v: %v", tt.words, err)
			continue
		}
		got := a.Record(1, 0x12, v3Pro8K)
		if got != tt.want {
			t.Errorf("%v: got %+v, want %+v", tt.words, got, tt.want)
		}
		if !a.Matches(got, v3Pro8K) {
			t.Errorf("%v: does not match its own record", tt.words)
		}
	}
}

func TestDialects(t *testing.T) {
	off, _ := Parse([]string{"off"}, v4Pro)
	if off.Record(1, 0x12, v4Pro) != record(1, 0x12, 0, 0, 0) {
		t.Error("V4 off should be function 0")
	}
	def, err := Parse([]string{"default"}, v4Pro)
	if err != nil || def.Record(1, 0x12, v4Pro) != record(1, 0x12, 0, 16, 1, 0) {
		t.Error("V4 default should be controller target 0")
	}
	if Decode(record(1, 0x12, 0, 16, 1, 0), v4Pro).String() != "default" {
		t.Error("V4 target 0 should decode as default")
	}
	if _, err := Parse([]string{"default"}, v3Pro8K); err == nil {
		t.Error("V3 Pro 8K has no separate default")
	}
}

func TestParseRejects(t *testing.T) {
	for _, words := range [][]string{
		nil,
		{"key"},
		{"key", "f13+f14"},
		{"key", "ctrl+shift"},
		{"key", "ctrl+ctrl+a"},
		{"key", "nosuchkey"},
		{"button", "z"},
		{"default", "extra"},
		{"macro", "x"},
	} {
		if _, err := Parse(words, v4Pro); err == nil {
			t.Errorf("%v accepted", words)
		}
	}
}

func TestRoundTripEveryName(t *testing.T) {
	_, targets, keys, _ := Names()
	for _, name := range targets {
		a, err := Parse([]string{"button", name}, v3Pro8K)
		if err != nil || Decode(a.Record(1, 0x10, v3Pro8K), v3Pro8K).String() != "button "+name {
			t.Errorf("button %s: %v", name, err)
		}
	}
	for _, name := range keys {
		a, err := Parse([]string{"key", name}, v3Pro8K)
		if err != nil || Decode(a.Record(1, 0x10, v3Pro8K), v3Pro8K).String() != "key "+name {
			t.Errorf("key %s: %v", name, err)
		}
	}
}

func TestButtons(t *testing.T) {
	if id, ok := ButtonID("M3"); !ok || id != 0x12 {
		t.Fatal("m3")
	}
	if ButtonName(0x3d) != "share" || ButtonName(0x99) != "0x99" {
		t.Fatal("names")
	}
}
