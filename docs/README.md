# Coe Docs

- [development.md](./development.md): source-build workflow, local release bundle installs, and module build commands.
- [configuration.md](./configuration.md): full config reference, defaults, and provider-specific examples.
- [install.md](./install.md): alpha installation flow with `systemd --user`, env file setup, and auto-managed GNOME shortcut fallback.
- [arch-install.md](./arch-install.md): Arch Linux `makepkg` installation flow, Fcitx5 packaging notes, and first-run checks.
- [architecture.md](./architecture.md): target architecture and the current shipped implementation split apart clearly.
- [fallbacks.md](./fallbacks.md): supported degraded paths, third-party binary policy, and shipped-vs-planned fallback behavior.
- [qwen3-asr-vllm.md](./qwen3-asr-vllm.md): running `Qwen/Qwen3-ASR-1.7B` on vLLM and wiring Coe to the local chat-completions endpoint.
- [feat/fcitx5-module-design.md](./feat/fcitx5-module-design.md): architecture decision for integrating Coe into Fcitx5 as a thin module plus daemon.
- [feat/fcitx-hold-to-talk-requirements.md](./feat/fcitx-hold-to-talk-requirements.md): requirement note for press-and-hold dictation semantics in the Fcitx5 module.
- [feat/fcitx5-implementation-plan.md](./feat/fcitx5-implementation-plan.md): execution plan for the Fcitx5 module, D-Bus contract, and rollout phases.
- [feat/selected-text-edit-requirements.md](./feat/selected-text-edit-requirements.md): requirement note for replacing selected text through voice editing in `runtime.mode: fcitx`.
- [gnome-focus-helper.md](./gnome-focus-helper.md): GNOME Shell extension contract for focus-aware paste shortcuts.
- [gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md): Verified and inferred GNOME support matrix for `GlobalShortcuts`.
