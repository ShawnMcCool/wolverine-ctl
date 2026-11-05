# wolverine-ctl

Configure a Razer Wolverine controller from Linux: polling rate and what the
extra buttons do. No Synapse, no Wine, no kernel driver changes.

The settings are written to the controller's onboard profile, so they stay
with the controller. They apply in every game and on every machine with
nothing running in the background.

```
$ wolverine-ctl status
Razer Wolverine V3 Pro 8K PC (2.4 GHz receiver)  /dev/hidraw7
  firmware  1.4.0
  serial    XX0000A00000000
  battery   87% (not charging)
  profile   1
  polling   2000 Hz
  m1        off
  m2        off
  m3        key f13
  m4        key f14
  m5        button a
  m6        off
  share     key alt+super+printscreen
```

## What it does

- Reads firmware version, serial number, battery, charging state, active
  profile, polling rate and every button assignment.
- Sets the polling rate: 250, 500, 1000, 2000, 4000 or 8000 Hz.
- Assigns M1–M6 and Share to a controller button, a keyboard key with
  modifiers, or nothing.
- Reads every change back from the controller and reports an error if the
  controller did not store it.

It talks to the controller's configuration interface (USB interface 1)
through Linux `hidraw`. The gamepad itself stays with the normal kernel driver
(`xpad`), so games and Steam keep working while you change settings.

## Supported controllers

| Controller | USB ID | Status |
|---|---|---|
| Wolverine V3 Pro 8K PC, USB cable | `1532:0a57` | Tested |
| Wolverine V3 Pro 8K PC, 2.4 GHz receiver | `1532:0a59` | Tested |
| Wolverine V4 Pro, PC HID mode (Function + Menu + Y) | `1532:0a79` | Same protocol, tested by [hyprdeck-razer](https://github.com/mikkeyboi/hyprdeck-razer); not tested with wolverine-ctl |

Not supported: the original Wolverine V3 Pro (`1532:0a3f`), which uses a
different protocol.
Find your controller's ID with `lsusb | grep 1532`. If you have one of these
and can test, please open an issue.

## Install

You need Go 1.24 or newer.

```sh
go install github.com/ShawnMcCool/wolverine-ctl@latest
```

This puts `wolverine-ctl` in `~/go/bin`. Or build from a clone:

```sh
git clone https://github.com/ShawnMcCool/wolverine-ctl
cd wolverine-ctl
make
sudo make install          # /usr/local/bin/wolverine-ctl
```

### Permissions (udev rule)

Without this, only root can open the controller. The rule gives the
logged-in user access to the configuration interface of supported
controllers, and nothing else:

```sh
sudo make install-udev
```

or by hand:

```sh
sudo install -m644 udev/70-wolverine-ctl.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw
```

Then unplug and replug the receiver.

### Gamepad driver

Linux 7.0 and newer bind the V3 Pro 8K PC to `xpad` automatically.
wolverine-ctl does not need the gamepad driver, but you need it to play.
Some packages blacklist `xpad` (for example `xone`). If the controller
doesn't work as a gamepad, check `modprobe -c | grep 'blacklist xpad'`.

## Usage

```
wolverine-ctl [--device PATH] [--profile N] COMMAND [ARGS]
```

| Command | What it does |
|---|---|
| `list` | List connected supported controllers |
| `status` | Show firmware, battery, polling rate and button assignments |
| `polling [RATE]` | Show the polling rate, or set it in Hz |
| `map BUTTON TARGET` | Assign a button: `off`, `button NAME` or `key CHORD` |
| `names` | List every button, target, key and modifier name |
| `version` | Print the version |

`TARGET` is one of:

| Target | Example | Effect |
|---|---|---|
| `off` | `map m6 off` | Does nothing (factory setting for M1–M6) |
| `button NAME` | `map m5 button a` | Presses a controller button: `a b x y lb rb lt rt l3 r3 view menu dpad-up dpad-down dpad-left dpad-right` |
| `key CHORD` | `map m3 key ctrl+shift+s` | Sends a key with optional modifiers: `ctrl shift alt super` (and `rctrl rshift ralt rsuper`) plus one key such as `a`, `5`, `f13`, `space`, `printscreen` |

Keys F13–F24 are useful for the paddles: no regular keyboard has them, so
they never clash with typing. Bind them in your desktop or game.

On the V4 Pro, `default` restores the firmware default. The V3 Pro 8K PC has
no separate default: its factory setting for M1–M6 is `off`.

Changes apply to the active onboard profile. Use `--profile N` to change
another profile; switch profiles on the controller itself.

### Examples

```sh
wolverine-ctl polling 8000
wolverine-ctl map m3 key f13
wolverine-ctl map m4 key ctrl+shift+s
wolverine-ctl map m5 button a
wolverine-ctl map m6 off
wolverine-ctl --profile 2 status
```

### Button positions (V3 Pro 8K PC)

| Button | Position |
|---|---|
| M1, M2 | Top edge, left and right of the bumpers |
| M3, M4 | Back paddles, left top and right top |
| M5, M6 | Back paddles, left bottom and right bottom |
| Share | Center button. Factory setting: `alt+super+printscreen` |

The paddles have no separate gamepad input. They either copy a controller
button or send a key, which is why keyboard keys are the way to make them
independent.

## Troubleshooting

**"controller did not respond; it may be asleep or off".** Wireless
controllers sleep after a few idle minutes. The receiver stays connected and
answers for them, but it can't reach the controller. Press a button and run
the command again.

**Cable and receiver both connected.** Both show up in `wolverine-ctl list`.
The controller answers only on the one it's using (the cable, when plugged
in), and wolverine-ctl picks that one automatically.

**"permission denied".** Install the udev rule and replug the receiver.

**"no supported controller connected".** Run `lsusb | grep 1532` and compare
the ID with the table above.

**Keys from the paddles show up as a keyboard.** That's expected: the
receiver has its own keyboard interface, and key assignments come from it.

## How it works and sources

The controller uses Razer's 90-byte HID feature-report protocol, the same
format as Synapse's `rzDevice25SagePC` controllers. [SOURCES.md](SOURCES.md)
lists every command, where it was documented, and whether it was tested on
real hardware.

Protocol details come from
[hyprdeck-razer](https://github.com/mikkeyboi/hyprdeck-razer) by mikkeyboi,
which first documented them for the Wolverine V4 Pro, and from Synapse's own
logs. wolverine-ctl is an independent implementation.

## Adding a controller or a command

- **Controller with the same protocol:** add an entry to `Models` in
  `internal/model/model.go` and its USB ID to `udev/70-wolverine-ctl.rules`.
  Run `wolverine-ctl status`, then record the result in SOURCES.md and the
  table above.
- **New command:** add a method in `internal/protocol/commands.go` with a test
  in `protocol_test.go`, a row in SOURCES.md, and, if users need it, an entry
  in the `commands` table in `main.go`.

```sh
make test
```

## Not affiliated with Razer

Razer and Wolverine are trademarks of Razer Inc. This project is not
affiliated with or endorsed by Razer. Changing onboard settings is at your
own risk. wolverine-ctl only sends commands documented in SOURCES.md and never
writes firmware.

## License

MIT. See [LICENSE](LICENSE).
