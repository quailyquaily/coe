# Coe Docs

- [install.md](./install.md): alpha installation flow with `systemd --user`, env file setup, and auto-managed GNOME shortcut fallback.
- [architecture.md](./architecture.md): target architecture and the current shipped implementation split apart clearly.
- [fallbacks.md](./fallbacks.md): supported degraded paths, third-party binary policy, and shipped-vs-planned fallback behavior.
- [feat/fcitx5-module-design.md](./feat/fcitx5-module-design.md): architecture decision for integrating Coe into Fcitx5 as a thin module plus daemon.
- [feat/fcitx-hold-to-talk-requirements.md](./feat/fcitx-hold-to-talk-requirements.md): requirement note for press-and-hold dictation semantics in the Fcitx5 module.
- [feat/fcitx5-implementation-plan.md](./feat/fcitx5-implementation-plan.md): execution plan for the Fcitx5 module, D-Bus contract, and rollout phases.
- [gnome-focus-helper.md](./gnome-focus-helper.md): GNOME Shell extension contract for focus-aware paste shortcuts.
- [gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md): Verified and inferred GNOME support matrix for `GlobalShortcuts`.
- [pw-record-exit-status.md](./pw-record-exit-status.md): root-cause note for `pw-record` returning exit status `1` on an intentional stop.
- [roadmap.md](./roadmap.md): delivered milestones, remaining portal work, package map, and risks.
