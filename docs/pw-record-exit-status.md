# `pw-record` Exit Status `1`

## Summary

On the validated GNOME Wayland machine for this repository, `pw-record` can produce valid audio data and still exit with status `1` when the recording is stopped intentionally.

This is not a Coe-specific crash.

It is a consequence of how PipeWire's `pw-cat` / `pw-record` tool handles `SIGINT` and `SIGTERM`.

## Local evidence

Local version:

- `pw-record`
- compiled with `libpipewire 1.0.5`
- linked with `libpipewire 1.0.5`

Reproduction used during investigation:

```bash
tmp=$(mktemp)
err=$(mktemp)
pw-record --rate 16000 --channels 1 --format s16 - >"$tmp" 2>"$err" &
pid=$!
sleep 2
kill -INT "$pid"
wait "$pid"
echo "exit=$?"
wc -c <"$tmp"
cat "$err"
```

Observed result on the real desktop session:

- `SIGINT`: `exit=1`, `bytes=61382`, empty `stderr`
- `SIGTERM`: `exit=1`, `bytes=61382`, empty `stderr`

That means the non-zero exit is compatible with a successful capture stop.

## Upstream source evidence

PipeWire upstream implements `pw-record` in `src/tools/pw-cat.c`.

Relevant behavior:

1. `exit_code` starts as `EXIT_FAILURE`
2. `SIGINT` and `SIGTERM` are both wired to `do_quit`
3. `do_quit` only calls `pw_main_loop_quit(data->loop)`
4. `exit_code` changes to `EXIT_SUCCESS` only if `data.drained` becomes true
5. `data.drained` is set in `on_drained`

In other words, an interrupt-driven stop quits the main loop but does not automatically mark the stream as drained, so the program returns failure even though audio bytes have already been emitted.

## Implication for Coe

Coe should not treat every `pw-record exit 1` as a hard failure.

The safe rule used in this repository is:

- if `pw-record` exits `1`
- and audio bytes were captured
- and `stderr` is empty

then the stop is treated as successful.

Coe still treats `exit 1` as an error when:

- no audio bytes were captured
- or `stderr` contains an actual error message

## Remaining caveat

This documents the validated behavior for the current PipeWire toolchain used during development.

It does not prove that every PipeWire version or distro packaging will behave identically, so future validation on newer versions is still useful.
