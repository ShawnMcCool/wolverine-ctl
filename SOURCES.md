# Sources

Where each protocol fact comes from. Every command wolverine-ctl sends is
listed here. When you add a command or a controller, add its evidence.

## References

- **hyprdeck-razer** — https://github.com/mikkeyboi/hyprdeck-razer (MIT,
  © 2026 mikkeyboi). Native Wolverine V4 Pro support, worked out from Razer
  Synapse's JavaScript modules for `rzDevice25` / `rzDevice25SagePC`. Files:
  `src/wolverine.rs` (framing, polling, battery), `src/wolverine_mapping.rs`
  (button list, assignment records, controller target IDs),
  `src/wolverine_keyboard.rs` (keyboard records), `src/wolverine_firmware.rs`
  (firmware version), `docs/controller-linux-support.md`. Read at commit
  `01eadd6` (2026-10-24). wolverine-ctl is a separate implementation; no code
  was copied.
- **Razer Synapse 4 logs** — the device log
  `products_2647_mw {15320A59-…}.log` from Synapse running under Wine
  (synapse-linux) names the device class `rzDevice25SagePC` for `1532:0a59`,
  and shows its requests: interface 1, report ID 10, e.g. Get Serial Number
  `22,0,130` (size 0x16, class 0x00, ID 0x82).
- **OpenRazer** — https://github.com/openrazer/openrazer. The same 90-byte
  report layout and status codes are used across Razer devices
  (`driver/razercommon.h`).
- **USB HID Usage Tables 1.5**, Keyboard/Keypad page (0x07) — key usages and
  the boot-keyboard modifier byte.
- **Hardware tests** — Wolverine V3 Pro 8K PC, firmware 1.4.0, over its
  2.4 GHz receiver (`1532:0a59`) and USB cable (`1532:0a57`), Linux 7.2 with
  `xpad` bound to interface 0, 2026-11-05. Results marked "verified" below.
  Every command in the table behaves the same on both identities. Wired
  interface 1 declares report 10 on vendor usage page `0xff00` (receiver:
  Generic Desktop); same 90-byte size. With the cable in, the receiver
  answers `0x04`.

## Transport

| Fact | Source |
|---|---|
| Configuration is a HID feature report on USB interface 1, report ID `0x0a`, 90 data bytes | Interface 1 report descriptor (`09 42 75 08 95 5a b2 02 01`); hyprdeck-razer; Synapse log |
| Layout `[status, txn, 3 reserved, size, class, id, args[80], crc, reserved]`, CRC = XOR of bytes 2..87 | OpenRazer `razercommon.h`; hyprdeck-razer `wolverine.rs`; verified |
| Transaction IDs cycle 1..31 | hyprdeck-razer (Synapse module 54849) |
| Status `0x02` OK, `0x01` busy, `0x03` failed, `0x04` no response, `0x05` unsupported | OpenRazer; verified `0x02`, `0x03`, `0x04` |
| `0x04` is returned while the controller sleeps; it recovers when woken | Verified |
| 30 ms pause before every transfer | Synapse log (`sleepTimeBetweenOut:30, sleepTimeBetweenOutIn:30, sleepTimeBetweenIn:30`). Verified: with 10 ms, a write sent within ~30 ms of a previous write's readback gets no reply and the receiver stops answering for about a second; with 30 ms, 11 consecutive write runs succeeded |

## Commands

| Command | Class/ID | Arguments → reply | Source | Status on 0a57 / 0a59 |
|---|---|---|---|---|
| Serial number | `00/82` | 22 zero bytes → ASCII | Synapse log; OpenRazer | Verified |
| Firmware version | `00/87` | 4 zero bytes → `[major, minor, internal, reserved, …]` | hyprdeck-razer `wolverine_firmware.rs` | Verified (1.4.0) |
| Active profile | `05/84` | `[0]` → `[profile]` | hyprdeck-razer | Verified |
| Polling rate get | `00/c0` | `[profile, 0]` → `[profile, period]` | hyprdeck-razer (Synapse modules 11880/38287) | Verified |
| Polling rate set | `00/40` | `[profile, period]` | hyprdeck-razer | Verified: 2000 Hz read back, `evtest` measured 1999–2000 reports/s |
| Battery | `07/80` | `[0, 0]` → `[0, level/255]` | hyprdeck-razer (Synapse modules 59737/50488) | Verified |
| Charging | `07/84` | `[0, 0]` → `[0, state]` (0 no, 1 yes, 4 fast) | hyprdeck-razer | Verified (0 on battery, 1 on cable) |
| Button list | `02/84` | 80 zero bytes → `[count, ids…]` | hyprdeck-razer (Synapse module 87969) | Verified: M1–M6 `0x10`–`0x15`, Share `0x3d` |
| Assignment get | `02/8c` | `[profile, button, 0, 0…]` (10 bytes) → record | hyprdeck-razer | Verified |
| Assignment set | `02/0c` | record (10 bytes) | hyprdeck-razer | Verified: keyboard keys and chords on M3–M6 and Share, controller button A on M1, off (target 0) |

Polling period is in 1/8000 s units: 1 = 8000 Hz, 2 = 4000, 4 = 2000,
8 = 1000, 16 = 500, 32 = 250 (hyprdeck-razer; Synapse module 14160 offers
these six rates for controllers).

## Assignment record

`[profile, button, mode, function, size, data[5]]`, mode 0 = normal
(hyprdeck-razer `wolverine_mapping.rs`, Synapse modules 10511/35922).

| Function | Meaning | Data | Source |
|---|---|---|---|
| 0 | Disabled (V4 Pro) | none | hyprdeck-razer. The V3 Pro 8K PC rejects it with status `0x04` (verified) |
| 16 | Controller button | `[target]`, size 1. Target 0: firmware default on the V4 Pro (hyprdeck-razer); disabled on the V3 Pro 8K PC | V3 Pro 8K PC: factory and Synapse "Disable" records are `10 01 00`, and those buttons send nothing in `evtest` (verified) |
| 2 | Keyboard key | `[modifiers, usage]`, size 2 | hyprdeck-razer `wolverine_keyboard.rs`; verified: Share's factory record `02 02 0c 46` = Alt+Super+PrintScreen |

Controller targets (firmware IDs, hyprdeck-razer / Synapse module 58701):
A 32, B 33, X 34, Y 35, LB 36, RB 37, LT 38, RT 39, L3 40, R3 41,
D-pad up/down/left/right 44–47, View 52, Menu 53.

A chord that is a single modifier is sent as that modifier's own key usage
(`0xe0`–`0xe7`) with an empty modifier byte (hyprdeck-razer
`wolverine_keyboard.rs`).

Paddle positions on the V3 Pro 8K PC (verified): M3 left top, M4 right top,
M5 left bottom, M6 right bottom.
