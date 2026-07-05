# Native Launch Policy

Agora DE can discover installed desktop applications before it can safely launch
all of them. Discovery is catalog metadata. Launch is session authority.

## Current Decision

Native `.desktop` `Exec` launch is deferred for shellui.

The installed den-k8 `compositorctl launch` binary exposes a `-cmd` option for
native process launch, and shellui already uses `compositorctl launch` for
webview targets. That host capability is not yet enough to mark imported
desktop entries launchable from the shell. The repo does not yet own a governed
native-launch boundary that expands desktop-entry arguments, controls
environment, records process ownership, associates the launch with a session,
waits for a matching surface, and reports failure evidence in a stable contract.

Until that boundary exists, shellui launchability means one of these:

- a catalog entry has an explicit shellui launch target in
  `go/internal/shellui/server`;
- that target launches a controlled webview URL/path with a deterministic title
  and app id;
- shellui waits for the expected compositor app id before returning success.

Imported installed entries from `.desktop` files are visible catalog entries,
but they are not launchable unless they also have an explicit shellui launch
target. The UI must render unsupported installed entries as non-launchable, and
`POST /api/catalog/launch` must fail for catalog ids with no explicit target.

## Desktop Entry Handling

`go/internal/appcatalog` may parse and normalize desktop-entry `Exec` values for
metadata only. The normalized token list is not execution authority.

Rules for the current importer:

- consume only the `[Desktop Entry]` group;
- import only exact, unlocalized keys used by the successor catalog;
- include only `Type=Application` entries in visible app projections;
- exclude `Hidden=true` and `NoDisplay=true` entries from visible views;
- normalize common desktop-entry field codes for future launch metadata;
- reject unsupported field codes for launchability metadata.

The importer must not invoke a shell, expand environment variables, resolve
relative programs, or guess missing launch context.

## Environment Assumptions

A future native launcher must make these choices explicit before shellui marks
installed apps launchable:

- process user and group;
- working directory;
- environment allowlist, including display and Wayland socket variables;
- session token and audit correlation id;
- logical output placement;
- app id, title, or other surface matching criteria;
- stdout, stderr, and exit-status capture policy;
- timeout and stale-launch cleanup behavior.

The default should be no shell evaluation. Desktop-entry tokens should become an
argv vector through a structured parser, not through string concatenation.

## Failure Behavior

For this series:

- imported entries with no explicit shellui launch target are disabled in the
  shell and omitted from launchable actions;
- direct launch requests for those entries return an error from shellui;
- parse failures during desktop-entry import skip the malformed entry instead
  of failing the whole catalog route;
- unsupported `Exec` field codes make an entry non-launchable as metadata even
  if the entry is still visible.

For a future native launcher, launch success must require more than process
spawn. It should return success only after the launch record can be associated
with an expected mapped surface or after a documented non-windowed launch state
is recorded.

## Follow-up Shape

Native launching should be added behind a small governed launcher service or
package, not by teaching shellui to execute arbitrary `Exec` strings directly.
That implementation can use the host compositor bridge capability once the repo
has tests and live evidence for the policy above.

The concrete first boundary design is `docs/native-launch-boundary-design.md`.
