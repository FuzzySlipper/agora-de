# Settings v1 operator and release guide

Settings v1 is one normal toplevel application with five independently loaded
modules: Displays, Window Management, Appearance, Shortcuts, and Diagnostics &
About. The catalog is first-party and build-time; it is not a third-party plugin
ABI or a generic configuration editor.

## Install and update

From the successor checkout:

```bash
./harness/ci/check-all.sh
deploy/shellui/install-user-services --enable --restart
```

The launcher entry and panel button both open `shell-settings`. Deep links use:

```text
http://127.0.0.1:17780/shell/dist/desktop/?surface=settings&module=<module-id>
```

Use `displays`, `window-management`, `appearance`, `shortcuts`, or
`diagnostics` as the module ID. An unavailable module stays visible with its
typed failure reason and does not prevent the other modules from loading.

## Stored configuration

User-owned state is under `~/.config/agora-de`:

- `appearance/theme-id` contains one validated bundled theme ID;
- `keybindings.toml` is the Agora-managed shortcut source;
- `displays/` contains only confirmed display profiles and reconciliation
  state;
- taskbar pins and saved layout state use their own bounded files in the same
  directory.

The managed shortcut block in `~/.config/wayfire.ini` is generated from
`keybindings.toml`. Settings preserves content outside its marked block. Do not
edit the generated block directly.

## Display safety and recovery

Display Apply first tests the complete topology, then starts an
authority-owned confirmation lease. Choose Keep only after the physical output
is usable. Revert restores the prior topology immediately. If Settings closes,
crashes, or cannot confirm before the countdown expires, the Rust display
authority reverts without UI cooperation and does not persist the candidate.

After a bad or interrupted display change:

1. wait for the confirmation timeout;
2. reopen Displays and reload authoritative state;
3. inspect the reconciliation status and connected-head identities;
4. use Restore Defaults to prepare a safe draft, then Apply and Keep only after
   confirming the physical output;
5. restart `agora-de-display-reconcile.service` if a confirmed profile was not
   re-applied after login.

Do not repair display state by writing `wayfire.ini` or calling `wlr-randr`;
those are not product authorities.

## Theme and shortcut recovery

Appearance persists only `agora-default` or `agora-ember`. If shell chrome is
not updated after Apply, restart the installed shell user services. Deleting an
invalid `~/.config/agora-de/appearance/theme-id` falls back to Agora Tide on
the next restart.

Shortcuts accepts only known managed binding IDs and Wayfire accelerators. It
cannot change commands or paths. `Super+Comma` opens Settings and is reserved so
the editor remains recoverable. Restore Defaults prepares the repository-owned
accelerators; Apply regenerates only the marked Wayfire block.

## Diagnostic export privacy

The Diagnostics support bundle is schema-owned and bounded to 16 KiB. It
contains the Settings schema/product version and fixed health records for the
shell gateway, compositor bridge, display authority, and settings contract. It
does not contain credentials, environment variables, arbitrary unit or path
queries, journal output, user document content or paths, window titles,
clipboard data, or governance logs.

## Known v1 limitations

- Display hotplug evidence depends on physically available hardware; simulated
  heads do not count as physical hotplug certification.
- Theme preview is local to the open Settings surface. Applying is persistent
  and returns an explicit shell-chrome restart requirement.
- Shortcuts edits the Agora-managed keymap only; unrelated Wayfire and
  application shortcuts are deliberately outside the module.
- Network, audio, Bluetooth, power, users, printers, application defaults, and
  third-party settings modules are outside v1.

## Contributor checklist

Read [Settings v1 Architecture](settings-v1-architecture.md) and the
[Settings Module Authoring Guide](settings-module-authoring.md). Every module
change must confirm:

- Rust-owned typed contracts and `protocol-codegen` output are current;
- new crates/packages are present in `governance/ownership.toml`;
- no TypeScript settings feature imports a sibling feature;
- client values are bounded IDs/enums rather than commands, paths, units, or
  generic JSON;
- stale revision, unavailable, validation, partial failure, rollback, and
  authoritative readback paths are tested as applicable;
- keyboard labels, focus, error announcements, reduced motion, contrast, and
  minimum target sizes are checked;
- focused gates plus `./harness/ci/check-all.sh` pass;
- installed-session evidence records literal commands, outcomes, restoration,
  hardware limitations, and the exact committed SHA when a clean release
  commit exists.

## 2026-07-18 validation packet

- Source base: `0d3ef437e4d43c6e643d3b1de48cb4ed58b622ee` plus the shared uncommitted
  Settings campaign worktree. This is not represented as an exact release SHA.
- Full gate: `./harness/ci/check-all.sh` passed.
- Deployment: `deploy/shellui/install-user-services --enable --restart`
  completed and the shell UI, panel, and display reconciliation user services
  were active.
- Displays: discovery, test/apply, Keep, explicit Revert, timeout rollback,
  restart persistence, and reconciliation were proven during tasks 5917-5920.
  Physical hotplug remained hardware-limited.
- Window Management: live compositor revision/readback was available; the
  module uses the existing layout authority and duplicate panel settings were
  removed.
- Appearance: Agora Tide to Agora Ember persisted across shell restart, then
  Agora Tide was restored and verified through `/api/theme`.
- Shortcuts: `focus_next` changed from `Super+J` to `Super+N` in the managed
  Wayfire block while the unrelated terminal binding remained; `Super+J` was
  restored.
- Diagnostics: the real four-component health set was available, the support
  bundle was 836 bytes, and the live overlay service toggled active then back
  to inactive.

The reviewer-session integration was unavailable for this campaign, per the
operator's instruction. Project CI and installed-service gates are the recorded
release checks; no independent-review result is claimed.
