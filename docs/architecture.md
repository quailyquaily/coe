# Coe Architecture

## 1. Goal

Build a Linux-native, Go-based dictation assistant that reproduces the core `koe` flow on **GNOME + Wayland** first:

1. Global hold-to-talk trigger.
2. Start microphone capture on key press.
3. Stop capture on key release.
4. Stream audio to ASR.
5. Send transcript to an LLM corrector.
6. Copy corrected text to the clipboard.
7. Optionally paste into the currently focused application.

The first release target is **GNOME on Wayland**. KDE and Hyprland are explicitly deferred until the GNOME path is stable.

## 2. Current shipped state

The repository currently ships and has been manually validated for this degraded-but-usable path:

1. User starts `coe serve`.
2. GNOME custom shortcut runs `coe trigger toggle`.
3. The daemon starts and stops `pw-record`.
4. Audio is wrapped as WAV and sent to OpenAI batch transcription.
5. Transcript is sent to OpenAI Responses for cleanup.
6. Final text is written to the clipboard through `wl-copy`.

What is implemented now:

- native capability probing through session D-Bus
- external-trigger fallback through a local Unix socket
- `pw-record` capture lifecycle
- OpenAI ASR client
- OpenAI LLM corrector
- `wl-copy` clipboard delivery
- portal-backed output session wiring for clipboard and auto-paste
- portal restore-token persistence for `RemoteDesktop` sessions

What is not implemented yet:

- portal-backed `GlobalShortcuts`
- validated `ydotool` fallback
- end-to-end validation of portal clipboard and portal auto-paste on a real GNOME target

This distinction matters because `doctor` can correctly detect portal availability even when the runtime implementation still uses command-line helpers.

## 3. Non-goals for v0

- No X11 parity target.
- No KDE- or Hyprland-specific implementation in the first milestone.
- No offline ASR/LLM in the first milestone.
- No polished GUI requirement in the first milestone. Tray/status UI is optional and secondary.
- No attempt to hide Wayland security prompts. If a portal requires authorization, the product accepts that.

## 4. Platform assumptions

The architecture assumes the following runtime environment:

- Linux desktop session with `XDG_SESSION_TYPE=wayland`
- GNOME desktop session
- Session D-Bus available
- `xdg-desktop-portal` and a GNOME backend present
- PipeWire available for microphone capture

Current bootstrap finding on the machine used for this repository:

- GNOME Wayland was present.
- `RemoteDesktop` and `Clipboard` were visible through the portal.
- `GlobalShortcuts` was not visible.

That means GNOME support must be treated as a distro/backend compatibility matrix, not a pure "GNOME implies portal parity" assumption.

The target architecture is still **portal-first**:

- Global shortcut: `org.freedesktop.portal.GlobalShortcuts`
- Clipboard write: `org.freedesktop.portal.Clipboard`
- Simulated paste: `org.freedesktop.portal.RemoteDesktop`

The current repository implementation is:

- Audio capture: `pw-record`
- Clipboard write: portal first, `wl-copy` fallback
- Paste injection: portal first, `ydotool` fallback

## 5. Confirmed constraints

These constraints shape the design:

- Wayland does not allow unrestricted global key capture by normal clients.
- Wayland does not allow unrestricted synthetic keyboard injection by normal clients.
- Portal support varies by compositor and backend implementation.
- Portal-based input injection may require explicit user authorization and should be treated as a runtime capability, not an assumption.

For GNOME specifically, the planning baseline is:

- `GlobalShortcuts` is a valid target interface and should be the default trigger path.
- `RemoteDesktop` is the correct first-class path for keyboard injection.
- `Clipboard` portal is the preferred first-class path for selection ownership / clipboard write.

## 6. Product behavior

### 6.1 Happy path

1. App starts and probes runtime capabilities.
2. App binds one global shortcut via `GlobalShortcuts`.
3. User presses the shortcut.
4. Audio recorder starts `pw-record` and reads PCM from stdout.
5. User releases the shortcut.
6. Recorder stops and finalizes the audio stream.
7. ASR client turns audio into raw transcript.
8. LLM corrector cleans punctuation / casing / formatting.
9. Output coordinator writes corrected text to the clipboard.
10. If auto-paste is authorized and enabled, output coordinator sends synthetic paste keystrokes.
11. App returns to idle.

### 6.2 Degraded modes

If the preferred runtime path is not available, the product degrades in this order:

1. Full portal mode:
   global shortcut + clipboard write + auto-paste
2. Clipboard-only mode:
   global shortcut + clipboard write, but no auto-paste
3. External-trigger mode:
   no internal global shortcut binding; user binds a GNOME shortcut that calls `coe trigger toggle`
4. Unsupported mode:
   app can run `doctor`, but `serve` should report why the runtime is not acceptable

Important nuance:

- External-trigger mode is intentionally degraded.
- It does not preserve true press/release hold-to-talk semantics on its own.
- On GNOME, the supported degraded behavior is toggle-style control over a local daemon socket.

Today, the repository implements only the third path in production use on GNOME 46-class systems.

## 7. Runtime capability model

Capability probing should classify the runtime into feature plans instead of a single yes/no verdict.

### 7.1 Features to probe

- Session type: Wayland vs other
- Desktop: GNOME vs other
- D-Bus session availability
- Portal interfaces:
  - `GlobalShortcuts`
  - `RemoteDesktop`
  - `Clipboard`
- External helpers:
  - `pw-record`
  - `wl-copy`
  - `wtype`
  - `ydotool`

### 7.2 Feature plans

Each feature gets a runtime plan:

- `portal`
- `command`
- `external-binding`
- `unavailable`

Examples:

- Hotkey:
  - `portal` when `GlobalShortcuts` exists
  - `external-binding` when the app must be triggered by a user-configured GNOME shortcut
- Audio:
  - `command` via `pw-record`
- Clipboard:
  - `portal` first
  - `command` via `wl-copy` fallback
- Auto-paste:
  - `portal` via `RemoteDesktop` first
  - `command` fallback later, not in the first GNOME milestone

## 8. Component architecture

### 8.1 Capability probe

Responsibility:

- Detect whether the current session matches the supported GNOME/Wayland target.
- Inspect portal interfaces.
- Detect presence of required helper binaries.
- Produce a runtime profile and human-readable notes.

Implementation note:

- The current skeleton introspects portal interfaces through a native Go D-Bus client.
- Session creation, binding, and signal handling for `GlobalShortcuts` are still pending.

### 8.2 Hotkey controller

Responsibility:

- Own the trigger binding lifecycle.
- Expose `Activated` and `Deactivated` events.
- Abstract the trigger source from the rest of the pipeline.

Interfaces:

- `PortalHotkeyService`
- `ExternalTriggerService`
- `NoopHotkeyService`

Fallback behavior:

- `ExternalTriggerService` is fed by local control commands such as `coe trigger toggle`.
- This is the supported GNOME fallback when `GlobalShortcuts` is missing.

### 8.3 Recorder

Responsibility:

- Start microphone capture on trigger activation.
- Stop cleanly on trigger release.
- Expose an audio stream or temp buffer to ASR.

Current implementation:

- Spawn `pw-record`
- Read PCM from stdout
- Stop child process on deactivation and tolerate non-fatal warning exits when audio data is already captured

### 8.4 ASR client

Responsibility:

- Convert recorded audio to transcript.
- Be provider-agnostic.

Current implementation:

- provider interface
- OpenAI batch transcription client
- WAV wrapping from raw `pw-record` PCM

### 8.5 LLM corrector

Responsibility:

- Normalize punctuation, spacing, paragraphing, and obvious ASR artifacts.
- Keep provider-specific transport isolated from the rest of the app.

Current implementation:

- provider interface
- OpenAI Responses-based correction
- transcript fallback if correction fails or returns empty text

### 8.6 Output coordinator

Responsibility:

- Write final text to clipboard.
- Optionally perform auto-paste.
- Handle partial success cleanly.

Rules:

- Clipboard success is enough to consider the request complete.
- Auto-paste failure must not discard clipboard success.
- Output stage should return structured status so the UX can explain what happened.

Current implementation:

- persistent portal-backed output session when the runtime exposes `Clipboard` and/or `RemoteDesktop`
- `wl-copy` fallback for clipboard delivery
- optional `ydotool` invocation for auto-paste fallback if configured

Planned implementation:

- portal `GlobalShortcuts`
- runtime validation and polishing for the portal output path

### 8.7 Session / auth store

Responsibility:

- Persist portal restore tokens if the desktop backend supports them.
- Store local state such as preferred trigger and output mode.

Current implementation:

- `RemoteDesktop` restore token is stored locally when portal persistence is enabled
- later runs reuse that token to avoid repeated authorization prompts when the backend accepts restoration

## 9. State machine

The runtime state machine should be explicit.

States:

- `idle`
- `arming`
- `recording`
- `transcribing`
- `correcting`
- `outputting`
- `error`

Transitions:

- `idle -> arming`
- `arming -> recording`
- `recording -> transcribing`
- `transcribing -> correcting`
- `correcting -> outputting`
- `outputting -> idle`
- Any state may transition to `error`
- `error -> idle` after reporting and cleanup

Important invariant:

- Only one dictation session runs at a time.

## 9. Error handling policy

- Failure to bind the global shortcut is startup-critical in full portal mode.
- Failure to access microphone is request-critical.
- Failure to call ASR or LLM is request-critical.
- Failure to auto-paste is not request-critical if clipboard write succeeded.

The app should distinguish:

- startup errors
- request errors
- degraded-but-usable runtime conditions

## 10. Security / privacy posture

- Do not persist raw audio by default.
- Keep audio in memory or short-lived temp files only when required by the provider contract.
- Keep provider API keys out of config files; use env var indirection.
- Treat clipboard and synthetic input as privileged output paths and make them user-visible in logs / status.

## 11. Package map

Planned Go package layout:

- `cmd/coe`: CLI entrypoint
- `internal/app`: app wiring
- `internal/config`: config load / defaults
- `internal/capabilities`: runtime probe and report
- `internal/platform/portal`: portal constants and probe helpers
- `internal/hotkey`: trigger abstractions
- `internal/audio`: recorder abstractions and `pw-record` adapter
- `internal/asr`: transcript provider interface
- `internal/llm`: correction provider interface
- `internal/output`: clipboard / auto-paste coordination
- `internal/pipeline`: dictation pipeline orchestration

## 12. Implementation choices locked in by this document

- Language: Go
- Desktop focus: GNOME on Wayland first
- Runtime style: portal-first
- Audio capture: `pw-record` first, native PipeWire binding deferred
- Initial D-Bus strategy: native Go D-Bus client from the start
- Success criterion for first alpha: clipboard-first, auto-paste second

## 13. External references

- `koe` repository: https://github.com/missuo/koe
- GlobalShortcuts portal docs: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.GlobalShortcuts.html
- RemoteDesktop portal docs: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html
- Clipboard portal docs: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Clipboard.html
- GNOME announcement mentioning desktop portal global shortcuts support on 2025-02-28: https://thisweek-gnome-org-79a21f.pages.gitlab.gnome.org/posts/2025/02/twig-189/
- PipeWire `pw-cat` / `pw-record` docs: https://docs.pipewire.org/page_man_pw-cat_1.html
