# wolverine-ctl

Go CLI (standard library only) that configures Razer Wolverine controllers
over Linux hidraw. Read README.md for usage and SOURCES.md for the protocol
and the evidence behind every command.

## Work in progress

Multi-session work is tracked in `docs/campaigns/` (index:
`docs/campaigns/README.md`). Resume a campaign by reading its file and
reconciling it with `git log` before changing code.

## Build, test, release

- `make test` (vet + tests), `make build`.
- CI (`.github/workflows/ci.yml`) checks gofmt, vet and tests on every push.
- Release: push an annotated `vX.Y.Z` tag. `release.yml` tests, builds
  amd64/arm64 binaries, publishes the GitHub release, then publishes the AUR
  package (`scripts/ship aur --ci`, secret `AUR_SSH_PRIVATE_KEY`).

## Code layout

- `internal/protocol`: report framing, transaction handling, one method per
  command (`commands.go`). New command → method + test + SOURCES.md row.
- `internal/model`: supported controllers and their dialects. New model →
  table entry + udev rule + README + SOURCES.md.
- `internal/mapping`: names ↔ assignment records.
- `main.go`: the `commands` table of subcommands.

## Testing on hardware

The maintainer's pad is a Wolverine V3 Pro 8K PC (wired `0a57`, receiver
`0a59`). Rules learned the hard way:

- Never send a command whose layout is not in SOURCES.md or a cited source.
  Read (get) before write (set).
- Status `0x04` means the pad is asleep or on the other connection. Ask the
  owner to press a button; don't retry blindly.
- Keep the 30 ms gap between transfers (`protocol.Client.Gap`).
- Put the pad back how you found it after a test, and say what you changed.
- Ask the owner before any write to a setting not already tested in SOURCES.md.
