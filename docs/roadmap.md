# Coe Roadmap

## 1. Scope order

The build order is intentionally narrow:

1. Documentation and package skeleton
2. Runtime capability probe (`doctor`)
3. GNOME + Wayland fallback trigger path
4. `pw-record` capture path
5. ASR provider
6. LLM corrector
7. Clipboard write
8. Portal hotkey and portal output
9. UX polish
10. KDE / Hyprland expansion

## 2. Milestones

### M0: repository skeleton

Deliverables:

- `/docs` architecture and roadmap
- Go module and package layout
- `coe doctor`
- `coe config init`
- `coe serve` skeleton that wires the runtime and reports the chosen plan

Exit criteria:

- `go test ./...` passes
- `go build ./...` passes

Status:

- complete

### M1: GNOME degraded alpha

Deliverables:

- External-trigger fallback via local daemon control socket
- Trigger events mapped to toggle / start / stop
- `pw-record` lifecycle control
- In-memory audio handoff

Exit criteria:

- On GNOME stacks without `GlobalShortcuts`, the app can still be triggered through `coe trigger toggle`
- Runtime reports degraded mode instead of crashing when capability is missing

Status:

- complete

### M2: text pipeline alpha

Deliverables:

- One ASR provider implementation
- One LLM provider implementation
- End-to-end transcript and correction flow

Exit criteria:

- Dictation produces corrected text
- Provider failures are reported cleanly

Status:

- complete

### M3: command-output alpha

Deliverables:

- `wl-copy` clipboard delivery
- clipboard-first success semantics
- optional command-based auto-paste hook

Exit criteria:

- Corrected text lands on the clipboard in the validated GNOME fallback flow
- Output failures are reported cleanly

Status:

- complete for clipboard
- partial for auto-paste fallback wiring

### M4: portal parity alpha

Deliverables:

- Native D-Bus integration for `GlobalShortcuts`
- Portal clipboard write
- Portal auto-paste when authorized
- Runtime selection between portal and command output paths

Exit criteria:

- On GNOME stacks that expose `GlobalShortcuts`, the app can start recording on press and stop on release
- Clipboard delivery uses the portal path when available
- Auto-paste can work through `RemoteDesktop` when authorized

Status:

- complete for portal clipboard and portal paste wiring
- pending real-session validation and `GlobalShortcuts`

### M5: productization

Deliverables:

- User-scoped install flow
- `systemd --user` service
- Better status UX
- Packaging / installation story
- Logs and diagnostics improvements

Status:

- restore token persistence implemented for portal output sessions
- user service packaging and install docs implemented
- remaining work is UX polish and broader install validation

### M6: compositor expansion

Deliverables:

- KDE support validation
- Hyprland support validation
- Runtime profile rules per compositor

## 3. Package responsibilities

### `cmd/coe`

- Parse CLI commands
- Initialize config
- Dispatch to `doctor`, `config init`, and `serve`

### `internal/config`

- Resolve config path
- Load config
- Write a default config file

### `internal/capabilities`

- Probe environment
- Summarize runtime plan
- Produce `doctor` output

### `internal/platform/portal`

- Hold interface constants
- Parse portal introspection output
- Host the native D-Bus probe today
- Provide the future home for portal session and signal handling

### `internal/hotkey`

- Define trigger events and hotkey service contracts
- Host the portal-backed implementation and the external-trigger fallback

### `internal/audio`

- Define recorder contract
- Hold `pw-record` adapter

### `internal/asr`

- Define transcript provider contract
- Host provider implementations

### `internal/llm`

- Define correction provider contract
- Host provider implementations

### `internal/output`

- Define output result model
- Coordinate clipboard and auto-paste behavior
- Prefer portal delivery when available and fall back to command helpers when needed

### `internal/pipeline`

- Orchestrate the dictation request lifecycle

### `internal/app`

- Wire config, capability probe, and pipeline
- Choose the runtime profile

### `internal/control`

- Host the local Unix socket server/client used by fallback trigger commands

## 4. Testing strategy

### Unit tests now

- OpenAI request/response parsing
- Config read / write
- Capability profile selection

### Integration tests next

- `doctor` against mocked portal introspection / property responses
- Recorder command construction
- Portal session lifecycle against mocked D-Bus responses
- Local control socket request / response behavior

### Manual validation for M1-M3

- GNOME Wayland session
- GNOME custom shortcut calling `coe trigger toggle`
- OpenAI transcript and correction quality
- Clipboard write into a native GTK app

## 5. Risks to manage

- Distro-to-distro differences in portal backend versions
- Some GNOME installations may expose `RemoteDesktop` and `Clipboard` but still miss `GlobalShortcuts`
- RemoteDesktop authorization UX may be worse than expected
- Auto-paste semantics can differ depending on focused app behavior
- `pw-record` output format details must match ASR provider expectations
- `pw-record` may return a non-zero exit status on an otherwise successful stop path, depending on how the process is terminated

## 6. Open questions for the next implementation slice

- Should OpenAI batch transcription remain the default first-party ASR path, or should we add a local/self-hosted provider next?
- Should the first LLM corrector be optional or always enabled?
- Should `coe trigger toggle` remain the default degraded GNOME fallback, or should we also add a dedicated one-shot dictation command?
- Once `GlobalShortcuts` is available on some GNOME targets, how should runtime preference and user override work between portal hold-to-talk and degraded toggle mode?
- Should the YAML schema stay hand-edited, or should we add comments/example generation for common desktop setups?
