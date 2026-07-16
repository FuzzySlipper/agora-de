# Native Launch Boundary Design

This design turns `docs/native-launch-policy.md` into the first implementation
boundary for installed `.desktop` application launch.

## Ownership Decision

The first native launcher should be a Go internal package:

```text
go/internal/nativelaunch
```

Reasoning:

- the current live compositor bridge and `agora-de-compositorctl launch` surface are Go;
- shellui is already a Go HTTP boundary and should call a narrow launcher
  interface instead of executing desktop entries itself;
- `go/internal/appcatalog`, `go/internal/session`, and
  `go/internal/launchlife` already model the pieces needed for a first safe
  adapter;
- Rust launch/session state remains the authority direction for generated
  contracts and longer-term governance, but native launch can land as a lifted
  Go cell while the compositor bridge is still Go.

When implemented, `governance/ownership.toml` must add the package before code
lands. The expected dependency shape is:

```text
go/internal/nativelaunch
  may import go/internal/appcatalog
  may import go/internal/launchlife
  may import go/internal/session
```

`shellui/server` may depend on `nativelaunch` only through a launch provider
adapter. `appcatalog` must not depend on `nativelaunch`.

## Contract Shape

The package should expose a small request/result API. Names are illustrative;
the implementation may adjust them while preserving the contract:

```go
type Request struct {
    Entry              appcatalog.Entry
    RequesterUID       int
    RequesterGID       int
    SessionToken       session.Token
    AuditCorrelationID string
    OutputName         string
    DesktopFilePath    string
    Now                time.Time
}

type Result struct {
    LaunchID  string
    SurfaceID string
    Status    Status
}
```

Required statuses:

- `launched`: process launch accepted and an expected mapped surface was found;
- `surface_observed_after_timeout`: no surface was found during the primary
  bounded wait, but a matching visible surface appeared during the short
  reconciliation window;
- `reused_existing_window`: process launch accepted and an already-visible
  surface matched the expected app id or title, as with browser process reuse;
- `launched_without_surface`: launch accepted for an explicitly non-windowed
  target;
- `rejected`: request failed validation before process launch;
- `failed`: launcher or compositor bridge reported failure after validation;
- `timed_out_no_surface`: process launch was accepted but no expected surface
  was observed before the bounded wait and reconciliation window expired.

Shellui should translate these into the existing `/api/catalog/launch` response
without learning how to parse or execute desktop-entry commands.

## Command Construction

The launcher must construct an argv vector. It must not invoke a shell and must
not hand a concatenated desktop-entry `Exec` string to a shell-like API.

The installed den-k8 `agora-de-compositorctl launch` currently exposes `-cmd string`.
Before native installed apps become launchable, the bridge/CLI contract must
prove that native launch accepts a structured argv vector, for example by adding
a repeated `--arg` or `--argv-json` option. If the only available bridge command
is an ambiguous command string, native installed entries stay non-launchable.

Field-code handling for plain launcher clicks:

- `%%`: literal `%`;
- `%c`: app display name as a single argument;
- `%k`: absolute desktop-file path as a single argument;
- `%i`, `%f`, `%F`, `%u`, `%U`: omitted because a launcher click supplies no
  icon arguments, selected files, or selected URLs;
- unknown field codes: reject before process launch.

Relative executable names resolve only through an explicit `PATH` allowlist.
Relative paths inside the desktop file are not expanded by shellui. Environment
variables inside `Exec` are not expanded.

## Environment And Working Directory

The launcher owns the process environment. The first allowlist should be small:

- `HOME`, `USER`, `LOGNAME`;
- `PATH` from the launcher policy, not inherited wholesale;
- `XDG_RUNTIME_DIR`;
- `WAYLAND_DISPLAY` and optionally `DISPLAY`;
- `DBUS_SESSION_BUS_ADDRESS`;
- `LANG` and `LC_*`;
- `XDG_CURRENT_DESKTOP`, `DESKTOP_SESSION`, `XDG_SESSION_TYPE`;
- `XDG_DATA_DIRS`.

Additional variables require tests that explain why they are needed. The default
working directory is the launching user's home directory. Desktop-entry `Path`
support must be added deliberately, validated as an absolute directory, and
tested before use.

## Session And Surface Association

Every native launch must carry:

- session token;
- requester uid and gid;
- audit correlation id;
- generated launch id;
- optional output placement request.

Success for windowed apps requires associating the launch record with a mapped
surface. Prefer bridge-owned launch id and PID ancestry association. App id and
title matching are hints, not authority. A launch should not be marked
`launched` only because a process was spawned.

The wait must be bounded. Timeout returns `timed_out_no_surface` with the launch
id so cleanup can still find the process. A short post-timeout reconciliation
window may classify a delayed surface as `surface_observed_after_timeout`, and
pre-existing surfaces may classify as `reused_existing_window` only when an
explicit app id or title hint matches.

Non-windowed launches are not part of the first visible desktop slice unless a
desktop entry is explicitly allowlisted as non-windowed and the result records
that no surface is expected.

## Shellui Integration

Shellui should keep the existing explicit webview launch targets for successor
shell apps. Installed entries become launchable only when a launcher provider
reports that the specific entry can be executed under this policy.

Initial UI/API behavior:

- fixture webview targets remain launchable;
- desktop-entry provider entries remain non-launchable by default;
- a native-launch allowlist may make selected installed entries launchable for
  live evidence;
- unsupported entries must remain visible but disabled.

Shellui must never execute `Entry.Exec` or `Entry.ExecTokens` directly.

The first wired provider name is `structured_agora-de-compositorctl`. It is disabled by
default and must be paired with an explicit desktop-entry id allowlist. The
provider sends repeated `--arg` values to agora-de-compositorctl and must not use
`--cmd`.

Installed configuration is environment-driven so native launch can be enabled
or rolled back without code edits:

```text
AGORA_DE_SHELLUI_CATALOG_PROVIDER=desktop_entries
AGORA_DE_SHELLUI_DESKTOP_ENTRY_ROOTS=/usr/share/applications:/home/agent/.local/share/applications
AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=structured_agora-de-compositorctl
AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST=Alacritty.desktop
AGORA_DE_SHELLUI_NATIVE_LAUNCH_UID=1001
AGORA_DE_SHELLUI_NATIVE_LAUNCH_GID=1002
AGORA_DE_SHELLUI_NATIVE_LAUNCH_SESSION_TOKEN=session-native-shell
AGORA_DE_SHELLUI_NATIVE_LAUNCH_OUTPUT=HDMI-A-1
AGORA_DE_SHELLUI_NATIVE_LAUNCH_HOME=/home/agent
AGORA_DE_SHELLUI_COMPOSITORCTL=/home/agent/.local/bin/agora-de-compositorctl
```

The allowlist matches the desktop-entry id exactly, for example
`Alacritty.desktop`. It does not match executable paths, display labels, icon
names, or category names. The special value `*` enables every installed entry
that the structured launcher can safely prepare; entries with unsupported field
codes remain visible but disabled. Unknown native launch provider values are
rejected at shellui startup. Rollback is setting
`AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER=disabled` and clearing
`AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST`.

Catalog disabled state is a contract, not only display copy. Non-launchable
installed entries carry `disabledCode` and `disabledReason`; current stable
codes are:

- `native_launch_disabled`: native launch is globally disabled;
- `native_launch_not_allowlisted`: the entry is preparable but not allowlisted;
- `unsupported_desktop_entry`: the desktop entry cannot be safely prepared;
- `native_launch_unavailable`: reserved for provider/config states that still
  cannot launch the entry.

The agora-de `go/cmd/agora-de-compositorctl` implementation now supports that narrow
structured launch contract directly:

```bash
agora-de-compositorctl launch \
  --arg /usr/bin/alacritty \
  --env WAYLAND_DISPLAY=wayland-1 \
  --env XDG_RUNTIME_DIR=/run/user/1001 \
  --session-token session-native \
  --audit-correlation-id native-smoke \
  --wait-surface
```

This command starts the argv vector without shell evaluation. Surface readback
now uses the same agora-de `agora-de-compositorctl` control-socket client as shellui.
The control socket is served by the agora-de compositor bridge daemon once
`deploy/compositor/install-compositor-bridge-service.sh` has replaced the
predecessor unit. The remaining runtime dependency is the Wayfire plugin loaded
by the active Wayfire session.

## Required Tests

Unit tests:

- argv construction rejects unknown, unsupported, and unterminated field codes;
- argv construction preserves literal arguments without shell expansion;
- `%c`, `%k`, and `%%` substitutions are deterministic;
- environment construction contains only allowlisted variables;
- working-directory validation rejects missing, relative, or non-directory paths;
- launch requests without session token, requester uid, or executable are
  rejected before bridge calls;
- timeout and bridge failure statuses are stable.

Integration tests:

- shellui marks desktop-entry apps non-launchable when no native provider is
  configured;
- shellui marks only allowlisted native entries launchable when the provider is
  configured;
- shellui launch calls the launcher provider, not `exec.Command` directly;
- fake compositor bridge receives structured argv, session, uid/gid, audit id,
  expected output, and wait parameters.

Live evidence before enabling by default:

- sidecar shellui with `--catalog-provider desktop_entries`;
- at least one allowlisted installed app launches on den-k8;
- `/api/catalog/apps` shows only the allowlisted installed app as launchable;
- `/api/catalog/launch` returns launch id and mapped surface id;
- `/api/surfaces` reports the mapped surface with matching launch id when the
  bridge provides it;
- physical output capture on `HDMI-A-1` reports `capture_visible`;
- close/stale-cleanup succeeds or the non-windowed status is explicitly
  recorded.
