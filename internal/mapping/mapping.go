// Package mapping translates between names users type (m3, button a,
// key ctrl+f13) and onboard assignment records. Sources are in SOURCES.md.
package mapping

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ShawnMcCool/wolverine-ctl/internal/protocol"
)

// Assignment record function codes.
const (
	functionDisabled   = 0
	functionKeyboard   = 2
	functionController = 16
)

// Programmable buttons by firmware ID. The controller reports which of these
// it has (02/84).
var buttons = map[string]byte{
	"m1": 0x10, "m2": 0x11, "m3": 0x12, "m4": 0x13, "m5": 0x14, "m6": 0x15,
	"share": 0x3d,
}

// Controller outputs by firmware ID.
var controllerTargets = map[string]byte{
	"a": 32, "b": 33, "x": 34, "y": 35,
	"lb": 36, "rb": 37, "lt": 38, "rt": 39, "l3": 40, "r3": 41,
	"dpad-up": 44, "dpad-down": 45, "dpad-left": 46, "dpad-right": 47,
	"view": 52, "menu": 53,
}

// Modifier bits of a keyboard assignment, in HID boot-keyboard order.
var modifiers = []struct {
	name    string
	aliases []string
}{
	{"ctrl", []string{"lctrl", "control"}},
	{"shift", []string{"lshift"}},
	{"alt", []string{"lalt"}},
	{"super", []string{"lsuper", "meta", "gui", "win"}},
	{"rctrl", nil},
	{"rshift", nil},
	{"ralt", []string{"altgr"}},
	{"rsuper", []string{"rmeta", "rgui"}},
}

// Keyboard keys by HID usage (Keyboard/Keypad page 0x07).
var keys = buildKeys()

func buildKeys() map[string]byte {
	k := map[string]byte{
		"enter": 0x28, "esc": 0x29, "backspace": 0x2a, "tab": 0x2b, "space": 0x2c,
		"minus": 0x2d, "equal": 0x2e, "leftbracket": 0x2f, "rightbracket": 0x30,
		"backslash": 0x31, "semicolon": 0x33, "apostrophe": 0x34, "grave": 0x35,
		"comma": 0x36, "period": 0x37, "slash": 0x38, "capslock": 0x39,
		"printscreen": 0x46, "scrolllock": 0x47, "pause": 0x48, "insert": 0x49,
		"home": 0x4a, "pageup": 0x4b, "delete": 0x4c, "end": 0x4d, "pagedown": 0x4e,
		"right": 0x4f, "left": 0x50, "down": 0x51, "up": 0x52,
		"kp0": 0x62,
	}
	for c := byte('a'); c <= 'z'; c++ {
		k[string(c)] = 0x04 + c - 'a'
	}
	for c := byte('1'); c <= '9'; c++ {
		k[string(c)] = 0x1e + c - '1'
		k["kp"+string(c)] = 0x59 + c - '1'
	}
	k["0"] = 0x27
	for n := 1; n <= 12; n++ {
		k[fmt.Sprintf("f%d", n)] = byte(0x3a + n - 1)
	}
	for n := 13; n <= 24; n++ {
		k[fmt.Sprintf("f%d", n)] = byte(0x68 + n - 13)
	}
	return k
}

// The modifier keys themselves (usages 0xe0..0xe7), used when a chord is a
// single modifier.
const modifierUsageBase = 0xe0

// Dialect holds per-model differences in how assignments are stored.
type Dialect struct {
	// TargetZeroDisables means a controller record with target 0 disables the
	// button and the controller rejects function 0. True on the V3 Pro 8K PC:
	// Synapse's "Disable" stores target 0, and function 0 returns status 0x04.
	// When false (V4 Pro, per hyprdeck-razer), target 0 is the firmware default
	// and function 0 disables.
	TargetZeroDisables bool
}

// Kind classifies an assignment.
type Kind int

const (
	Default    Kind = iota // firmware default
	Disabled               // the button does nothing
	Controller             // presses a controller button
	Keyboard               // sends a key with optional modifiers
	Unknown                // a record this program does not decode
)

// Assignment is a decoded onboard button assignment.
type Assignment struct {
	Kind      Kind
	Target    byte // Controller
	Modifiers byte // Keyboard
	Key       byte // Keyboard
	record    protocol.Record
}

// Decode interprets a record read from the controller.
func Decode(r protocol.Record, d Dialect) Assignment {
	a := Assignment{Kind: Unknown, record: r}
	switch {
	case r.Function == functionDisabled:
		a.Kind = Disabled
	case r.Function == functionController && r.Size == 1 && r.Data[0] == 0:
		a.Kind = Default
		if d.TargetZeroDisables {
			a.Kind = Disabled
		}
	case r.Function == functionController && r.Size == 1:
		a.Kind, a.Target = Controller, r.Data[0]
	case r.Function == functionKeyboard && r.Size == 2:
		a.Kind, a.Modifiers, a.Key = Keyboard, r.Data[0], r.Data[1]
	}
	return a
}

// Record encodes the assignment for a profile and button.
func (a Assignment) Record(profile, button byte, d Dialect) protocol.Record {
	r := protocol.Record{Profile: profile, Button: button}
	switch a.Kind {
	case Default:
		r.Function, r.Size = functionController, 1
	case Controller:
		r.Function, r.Size, r.Data[0] = functionController, 1, a.Target
	case Keyboard:
		r.Function, r.Size, r.Data[0], r.Data[1] = functionKeyboard, 2, a.Modifiers, a.Key
	case Disabled:
		if d.TargetZeroDisables {
			r.Function, r.Size = functionController, 1
		}
	case Unknown:
		r = a.record
		r.Profile, r.Button = profile, button
	}
	return r
}

// Matches reports whether a record read back stores this assignment.
func (a Assignment) Matches(r protocol.Record, d Dialect) bool {
	return a.Record(r.Profile, r.Button, d) == r
}

func (a Assignment) String() string {
	switch a.Kind {
	case Default:
		return "default"
	case Disabled:
		return "off"
	case Controller:
		return "button " + nameOf(controllerTargets, a.Target)
	case Keyboard:
		return "key " + chordName(a.Modifiers, a.Key)
	default:
		r := a.record
		return fmt.Sprintf("unknown (function %d, data % x)", r.Function, r.Data[:r.Size])
	}
}

// Parse reads a target from command-line words:
// default | off | button NAME | key CHORD.
func Parse(words []string, d Dialect) (Assignment, error) {
	if len(words) == 0 {
		return Assignment{}, fmt.Errorf("missing target: default, off, button NAME or key CHORD")
	}
	kind := strings.ToLower(words[0])
	switch {
	case kind == "default" && len(words) == 1:
		if d.TargetZeroDisables {
			return Assignment{}, fmt.Errorf("this controller has no separate firmware default; use off")
		}
		return Assignment{Kind: Default}, nil
	case (kind == "off" || kind == "disable" || kind == "disabled") && len(words) == 1:
		return Assignment{Kind: Disabled}, nil
	case kind == "button" && len(words) == 2:
		target, ok := controllerTargets[strings.ToLower(words[1])]
		if !ok {
			return Assignment{}, fmt.Errorf("unknown controller button %q; run 'wolverine-ctl names'", words[1])
		}
		return Assignment{Kind: Controller, Target: target}, nil
	case kind == "key" && len(words) == 2:
		mods, key, err := parseChord(words[1])
		if err != nil {
			return Assignment{}, err
		}
		return Assignment{Kind: Keyboard, Modifiers: mods, Key: key}, nil
	}
	return Assignment{}, fmt.Errorf("invalid target %q: use default, off, button NAME or key CHORD", strings.Join(words, " "))
}

// parseChord reads "ctrl+shift+f13": optional modifiers and one key. A lone
// modifier ("ctrl") is sent as that modifier key.
func parseChord(chord string) (byte, byte, error) {
	var mods byte
	var key byte
	haveKey := false
	for _, part := range strings.Split(strings.ToLower(chord), "+") {
		if bit, ok := modifierBit(part); ok {
			if mods&bit != 0 {
				return 0, 0, fmt.Errorf("modifier %q appears twice in %q", part, chord)
			}
			mods |= bit
			continue
		}
		usage, ok := keys[part]
		if !ok {
			return 0, 0, fmt.Errorf("unknown key %q in %q; run 'wolverine-ctl names'", part, chord)
		}
		if haveKey {
			return 0, 0, fmt.Errorf("%q has more than one key; a shortcut is one key plus modifiers", chord)
		}
		key, haveKey = usage, true
	}
	switch {
	case haveKey:
		return mods, key, nil
	case mods != 0 && mods&(mods-1) == 0:
		for i := 0; i < 8; i++ {
			if mods == 1<<i {
				return 0, byte(modifierUsageBase + i), nil
			}
		}
	}
	return 0, 0, fmt.Errorf("%q needs a key, for example ctrl+shift+f13", chord)
}

func modifierBit(name string) (byte, bool) {
	for i, m := range modifiers {
		if name == m.name {
			return 1 << i, true
		}
		for _, alias := range m.aliases {
			if name == alias {
				return 1 << i, true
			}
		}
	}
	return 0, false
}

func chordName(mods, key byte) string {
	var parts []string
	for i, m := range modifiers {
		if mods&(1<<i) != 0 {
			parts = append(parts, m.name)
		}
	}
	if key >= modifierUsageBase && key < modifierUsageBase+8 {
		parts = append(parts, modifiers[key-modifierUsageBase].name)
	} else {
		parts = append(parts, nameOf(keys, key))
	}
	return strings.Join(parts, "+")
}

func nameOf(table map[string]byte, value byte) string {
	for name, v := range table {
		if v == value {
			return name
		}
	}
	return fmt.Sprintf("0x%02x", value)
}

// ButtonID returns the firmware ID of a programmable button name.
func ButtonID(name string) (byte, bool) {
	id, ok := buttons[strings.ToLower(name)]
	return id, ok
}

// ButtonName returns the name of a firmware button ID.
func ButtonName(id byte) string { return nameOf(buttons, id) }

// Names lists every accepted name, for the 'names' command.
func Names() (buttonNames, targetNames, keyNames, modifierNames []string) {
	return sortedKeys(buttons), sortedKeys(controllerTargets), sortedKeys(keys), modifierList()
}

func modifierList() []string {
	var out []string
	for _, m := range modifiers {
		out = append(out, m.name)
	}
	return out
}

func sortedKeys(m map[string]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
