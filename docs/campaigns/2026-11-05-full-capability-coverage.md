---
status: planning
opened: 2026-11-05
last_updated: 2026-11-05
---
# Campaign: Full capability coverage for the Wolverine V3 Pro 8K PC

## Glossary

| Term | Meaning |
|---|---|
| **pad** | The physical controller, a Razer Wolverine V3 Pro 8K PC. |
| **connection** | One USB identity the pad is reached through: wired `1532:0a57` or the 2.4 GHz receiver `1532:0a59`. |
| **configuration interface** | USB interface 1 of a connection. Its hidraw node carries the configuration feature report. |
| **command** | One request/reply over the configuration interface, named by its class and ID, e.g. `05/84`. |
| **get command** | A command that only reads. By Razer convention its ID has the high bit set (`0x8X`, `0xCX`). |
| **set command** | A command that changes stored state. Usually the matching get ID without the high bit (`05/84` → `05/04`). |
| **onboard profile** | One of the pad's stored setting sets (the product page says 4). Settings are per profile. |
| **active profile** | The onboard profile currently in effect. |
| **assignment** | The stored behaviour of one programmable button (M1–M6, Share) in one profile: a 10-byte record. |
| **dialect** | Per-model differences in how a setting is encoded (`mapping.Dialect`), e.g. how "off" is stored. |
| **capability** | One user-facing setting or reading the pad supports, such as polling rate, sleep timer or active profile. |
| **capability inventory** | `docs/capabilities.md`: every capability with its commands, evidence and support status. |
| **Synapse** | Razer Synapse 4, Razer's Windows configuration app, the reference for which capabilities exist. |
| **device module** | The JavaScript code inside Synapse that implements one device family (`rzDevice25SagePC` for this pad). |
| **verified** | Tested on the pad with wolverine-ctl: a write is read back, and the effect is observed where observable. |

## Goal

Make wolverine-ctl configure everything Razer Synapse can configure on the
Wolverine V3 Pro 8K PC, so Linux users never need Synapse for this pad. v0.1
covers polling rate and button assignments. The pad also has onboard
profiles, a sleep timer and power saving, and possibly stick, trigger and
HyperShift settings. Each capability is either supported and verified, or
excluded with a written reason.

## Status

Planning. v0.1.1 is released (polling, assignments, status, AUR pipeline). No
milestone started. Next: decide open decision 1, then M1.

## Principles

1. **Evidence before bytes.** No command is sent to the pad until its class,
   ID and argument layout come from a source (Synapse's device module or log,
   hyprdeck-razer, OpenRazer) recorded in SOURCES.md. No guessing.
2. **Read before write.** A new capability's get command is verified before
   its set command is ever sent.
3. **Every write is read back,** and the user-visible effect is measured where
   possible (`evtest` for input, a stopwatch for timers).
4. **Restore after testing.** Hardware tests leave the pad as they found it,
   unless the owner chose the new value.
5. **One capability, one row** in `docs/capabilities.md` and SOURCES.md.
6. **Per-model differences live in the dialect or the model table,** never as
   `if productID == …` in commands.
7. **No firmware writes and no calibration writes** without a separate,
   explicit owner decision.
8. **Each milestone ends in a release** (tag push → GitHub release + AUR).

## Open decisions

1. **How to get Synapse's device module and setting list for research.**
   - (a) Reinstall synapse-linux in a throwaway Wine prefix. Harvest the
     downloaded JS (`rzDevice25SagePC` and its UI schema) plus its logs while
     clicking through every settings page, then delete the prefix.
   - (b) A Windows VM with the pad passed through, plus `usbmon` captures of
     Synapse traffic.
   - (c) Probe get commands blindly on the pad.

   Recommendation: (a). It is cheap and already proven on this machine, and
   the JS gives command layouts directly; under Wine the pad's commands also
   work once the pad is awake. (b) only if (a) cannot reach a setting.
   Reject (c): T13nou's V3 Pro battery tool warns that some vendor commands
   reset or power off the pad.
2. **A raw command for research.** A `wolverine-ctl raw CLASS ID [ARGS…]`
   subcommand that sends one command with correct framing, transaction ID and
   30 ms timing. Recommendation: yes, documented as an expert tool and
   limited to get commands unless `--write` is given. The ad-hoc Python probe
   used in session 1 sent malformed commands twice when an argument was left
   out.
3. **Profile backup and restore** (`export` / `import` of every setting in a
   profile as a file). Recommendation: in scope as the last milestone. It
   reuses every capability's get/set and is the main thing users want before
   experimenting.
4. **Calibration.** Recommendation: exclude. It needs live stick input and a
   guided flow, and a bad write could leave the sticks miscalibrated.
   Firmware updates: exclude (Principle 7).
5. **Machine-readable output** (`status --json`). Recommendation: defer
   until someone asks.

## Decisions made

* `2026-11-05` — Campaign opened in the tool's repository (`docs/campaigns/`).

## Milestones

| | | |
|---|---|---|
| M1 | Capability inventory: `docs/capabilities.md` lists every setting Synapse offers for `0a57`/`0a59`, with its command, argument layout, source and status | open |
| M2 | `raw` research command (if decision 2 is yes) | open |
| M3 | Onboard profiles: list, show active, switch active (`05/81`, `05/84`, `05/04`); `--profile` everywhere already works | open |
| M4 | Power: sleep timer on battery, power saving (enabled + value) | open |
| M5 | Remaining settings found by M1 (sticks, triggers, HyperShift, …), each as its own milestone once M1 sizes them | open |
| M6 | Profile export/import (if decision 3 is yes) | open |

## Completion criteria

* Every row in `docs/capabilities.md` for the V3 Pro 8K PC reads
  **supported (verified)** or **excluded: <reason>**.
* README documents every supported capability with an example.
* SOURCES.md has a row with evidence for every command the tool sends.
* The final milestone is released, and the AUR package is at that version.
* `~/knowledge/devices/razer-wolverine-v3-pro-8k.md` is updated (outside this
  repo, owner's knowledge base).

## Known from session 1 (2026-11-05)

These facts come from Synapse logs and hardware tests that no longer exist on
disk: the Synapse prefix was deleted. They are the starting point for M1.

**Synapse device log (`products_2647_mw …log`, synapse-linux, 2026-11-03).**
Commands Synapse sent to this pad, by its own description (all got status
`0x04` then because the pad was asleep):

Descriptions Synapse logged for this pad, and how often (the log was
truncated at the description on some lines, so the pairing of a description
with a command is **confirmed** only where marked):

| Synapse description | Count | Command |
|---|---|---|
| Get Serial Number | 21 | `00/82`, size 22 — confirmed |
| Get Device Charging Status | 220 | `07/84` — confirmed (hyprdeck-razer, verified) |
| Get Battery Level | 220 | `07/80` — confirmed (hyprdeck-razer, verified) |
| Get Button ID list | 42 | `02/84` — confirmed (hyprdeck-razer, verified) |
| Get Edition Information | 61 | unknown |
| Get Time to Sleep Mode on Battery Mode | 41 | inferred `07/83`, size 2 |
| Set Time to Sleep Mode on Battery Mode | 21 | inferred `07/03`, size 2; one logged request carried data `[1, 44]` (0x012c = 300, likely seconds = 5 min) |
| Get Device Mode | 23 | inferred `00/84`, size 2 |

Raw command headers seen in the log, `[size, class, id]`: `[22,0,130]`,
`[3,0,134]`, `[2,0,132]`, `[2,7,131]`, `[2,7,132]`, `[2,7,128]`, `[2,7,3]`,
and one 80-byte request. `00/86` (size 3) has no confirmed description.

Synapse UI actions seen for this pad: `ON_POLLING_RATE`
(`pollingRateWireless: 2000`), `ON_SET_POWER_SAVING`
(`powerSaving: {isEnabled: true, value: 5}`), `ON_SET_KEYMAPPING`
(`inputType: ControllerInput`, `inputID: BUTTON_M3`,
`outputType: keyboardGroup`, `isHyperShift: false`). Runtime data:
`hasBattery: true`, `isChromaDevice: false`,
`IS_HYPER_POLLING_PAIRED: true`. The state machine lists generic slots
(`pollingRate`, `keyMapping`, `power`, `brightness`, …); most are generic to
all Razer devices, not proof of capability.

**hyprdeck-razer documents** (Wolverine V4 Pro, same SagePC family):
profile list `05/81` (80 bytes → `[count, ids…]`), active profile set
`05/04 [profile]`, motor scaling `0c/86` (the 8K has no motors).

**Hardware lessons.**
- The receiver answers `0x04` while the pad sleeps; press a button.
- Wait 30 ms before every transfer, or back-to-back writes stall the receiver.
- The 8K rejects function 0 ("disabled"); disable is controller target 0.
- With the cable in, the receiver answers `0x04`; the wired connection is
  used.
- The pad's factory record for Share is keyboard Alt+Super+PrintScreen.

## Pointers

- `SOURCES.md` — every verified command and its evidence.
- `internal/protocol/commands.go` — one method per command.
- `internal/model/model.go` — the model table and dialects.
- hyprdeck-razer: https://github.com/mikkeyboi/hyprdeck-razer (fork at
  `~/src/hyprdeck-razer`, PR #3 adds the 8K). Its
  `docs/controller-linux-support.md` cites Synapse module numbers.
- synapse-linux (Wine Flatpak used in session 1):
  https://github.com/adds39939/synapse-linux. Install and udev steps are in
  `~/knowledge/devices/razer-wolverine-v3-pro-8k.md`.
- OpenRazer report layout: `driver/razercommon.h` in
  https://github.com/openrazer/openrazer.
