# Native Launch Boundary Design

This design turns `docs/native-launch-policy.md` into the first implementation
boundary for installed `.desktop` application launch.

## Ownership Decision

The first native launcher should be a Go internal package:

```text
go/internal/nativelaunch
```

Reasoning:

- the current live compositor bridge and `compositorctl launch` surface are Go;
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
- `launched_without_surface`: launch accepted for an explicitly non-windowed
  target;
- `rejected`: request failed validation before process launch;
- `failed`: launcher or compositor bridge reported failure after validation;
- `timed_out`: process launch was accepted but no expected surface was observed
  before the bounded wait expired.

Shellui should translate these into the existing `/api/catalog/launch` response
without learning how to parse or execute desktop-entry commands.

## Command Construction

The launcher must construct an argv vector. It must not invoke a shell and must
not hand a concatenated desktop-entry `Exec` string to a shell-like API.

The installed den-k8 `compositorctl launch` currently exposes `-cmd string`.
Before native installed apps become launchable, the bridge/CLI contract must
prove that native launch accepts a structured argv vector, for example by adding
a repeated `--arg` or `--argv-json` option. If the only available bridge command
is an ambiguous command string, native installed entries stay non-launchable.

Field-code handling for the first implementation:

- `%%`: literal `%`;
- `%c`: app display name as a single argument;
- `%k`: absolute desktop-file path as a single argument;
- `%i`: either structured icon arguments according to the desktop-entry spec or
  a documented unsupported-field rejection;
- `%f`, `%F`, `%u`, `%U`: unsupported until shellui can supply selected files or
  URLs intentionally;
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

The wait must be bounded. Timeout returns `timed_out` with the launch id so
cleanup and later reconciliation can still find the process or surface.

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

The first wired provider name is `structured_compositorctl`. It is disabled by
default and must be paired with an explicit desktop-entry id allowlist. The
provider sends repeated `--arg` values to compositorctl and must not use
`--cmd`.

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
