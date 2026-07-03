# Lesson Packet: Agora Desktop Concern (predecessor: agora-os GUI/DE/WM components)

*Successor-pattern lesson packet per `den:patch/successor-pattern`. The predecessor
implementation is evidence, not property. This packet records what the desktop
concern inside agora-os taught us: required behavior, contracts worth inheriting,
operational constraints, failure modes, real consumers, and the accidental
structure the successor must not inherit. Companion doc: `successor-brief.md`.*

Status: drafted 2026-07-02 against agora-os @ `ba4b523`.

---

## 1. Predecessor scope (what is being succeeded)

| Component | Location | Size / language | Condition |
|---|---|---|---|
| Wayfire enforcement plugin | `compositor/wayfire-plugin/` | ~3.2k lines C++ (`plugin.cpp` 2,159; `protocol.hpp` 920) | Working; 4–7x over the 300–500-line "thin shim" budget from `research/compositor-decision.md` |
| Compositor bridge daemon | `cmd/compositor-bridge`, `internal/compositor` | ~5.5k lines Go + 3.5k tests | Working; `bridge.go` is a 4,460-line god file (sessions, launch lifecycle, PID-ancestry association, capture, input inject, policy, layout, publishing under one type/mutex) |
| a11y readback | `internal/compositor/a11y.go` | 509 lines Go | Working |
| Root control CLI | `cmd/compositorctl` | ~2.9k lines Go | Working; also carries shell-admin subcommands (`shell.go`) |
| Webview launcher | `cmd/webview-launcher`, `internal/webview` | Go + 11KB Python GI helper (`helper.py`, GTK 3.0) | Working; two-language subprocess design is a known fragility |
| Browser bus gateway | `cmd/event-bus-web`, `internal/webbus` | Go | Working; token model is one of the strongest assets |
| Shell backend-for-frontend | `internal/shellui` | 1,768-line `server.go` + 71KB tests | Working; god file (see §7) |
| App catalog / defaults | `internal/appcatalog`, `internal/shelldefaults` | small Go | Healthy; move as-is |
| Shell frontends | `shell/` | ~5k lines TS, two apps | Operator console (`shell/src/app.ts`, 1,235-line monolith) + desktop shell (`shell/desktop/`, widget-factored). `strict: false`, no lint, no boundaries, `tsc`+`cp` build, hand-rolled `.mjs` tests |
| Shell chrome + host deploy | `deploy/den-k8plus/` | bash + Python | `agora-shell-panel-supervisor` (mode dispatch), `native-shell-dock` (GTK/GtkLayerShell dock) — product code living in the deploy directory |
| GUI protocol types | `internal/schema/compositor.go` (1,087), `layout.go` (407), `shell.go` | Go | ~half the schema package by volume is GUI-owned |
| Selection environment | `test/phase2.sh`, `test/phase3.sh`, `test/phase3probe`, `scripts/vm.sh` GUI paths, `shell/desktop/visual-contracts/` | bash/Go/fixtures | Must travel with the successor |

Stays in agora-os: isolation, admin-agent, audit, event-bus broker, agent-supervisor,
agentsim/ambassador/llm/r2worker (3PO/R2), audit-ebpf, core `schema`, `peercred`.

## 2. Required behavior (the contract the successor must satisfy)

### Compositor mediation
- Every surface is attributed to a kernel-attested uid at map time
  (`wl_client_get_credentials` via the plugin today). Attribution is unspoofable;
  payload-claimed identity is never trusted.
- Surface lifecycle events (map/unmap/focus/tile/minimize/fullscreen/above,
  geometry, stacking, workspace) stream to the bridge and are published on the
  bus under `compositor.surface.*`. The bridge is the authoritative source;
  `webview-launcher`'s self-reported `compositor.surface.*` messages are
  advisory convenience only (this dual-source ambiguity is a defect to fix, see §7).
- Per-surface input deny is enforced synchronously in-compositor against a local
  policy cache; policy authority lives out-of-process in Go and pushes updates
  async. Per-event checks are O(1) local lookups, never IPC.
- Forced surface close, focus/raise/move/resize/tile/always-on-top/fullscreen/
  maximize/minimize controls, with per-action allow/deny policy and denied-action
  events published for audit.
- Input context switching (which actor uid owns input) is a root-approved control.
- Per-surface pixel capture with visual-inspection classification; capture
  readback evidence is recorded separately from presentation evidence (§4).
- Root-approved viewport grants recorded to an append-only log.

### Launch and session model
- Launches happen as the requesting peer's uid (`launchAppAsPeer`), inside
  bridge-issued sessions with session tokens. Cleanup (terminate, session
  destroy) requires the session token.
- Launch→surface association: PID ancestry walk (`/proc` parent chain) plus
  app-id/title hints; surfaces are reconciled to launches after the fact because
  Wayland gives no direct launch→surface link.
- `--wait-surface` semantics: a launch can block until its surface is mapped and
  settled, with bounded timeout and `app_not_ready` as a retryable outcome
  (bounded retry, preserve every returned launch handle, never unbounded loops).

### Browser gateway and tokens
- WebSocket auth: `Authorization: Bearer` for programmatic clients,
  `Sec-WebSocket-Protocol: agora.token.<token>` for browsers. Query-parameter
  tokens intentionally unsupported. Shell reads its token from the URL fragment
  and stores it in localStorage; tokens never travel as query params.
- Published events are stamped with the authenticated uid via the trusted
  root-owned bus connection; subscriptions are filtered by identity.
- Reserved namespaces: `webview.broadcast.*` (shared), `webview.inbox.<uid>.*`
  (uid-scoped). Human-shell tokens get the full feed.
- Origins: strict same-origin default, `AGORA_WEBBUS_ALLOWED_ORIGINS` allowlist.
- Human tokens minted out-of-band (`event-bus-web mint-token --human`).

### Shell surfaces
- Operator console: agent/surface/escalation state snapshot, live audit tail,
  grant recording, escalation decide, per-surface action controls.
- Desktop shell: taskbar/launcher, clock, agent health, work-surface controls,
  Command Center, notifications, window chrome; theme-driven visuals.
- App catalog (`internal/appcatalog`) with .desktop import; catalog-driven
  launch through the shell with policy-checked deny responses.
- Widget system: packaged widgets installed by `compositorctl shell
  install-defaults`, served through `/api/shell/widget-proxy/<id>/`, publishing
  postMessage events that the shell prefixes onto the bus as `widget.<id>.*`.
- Theme packages: manifest at `/api/shell/theme.json`; runtime selection under
  `${SHELL_CONFIG_DIR:-/var/lib/agora-shell}/theme-selection.json`; bundled
  fallback `agora-default` ("Agora Observatory"). Only validated `--agora-*`
  tokens (or explicitly enabled `extension.*`) accepted; theme CSS passes a
  server-side safe-visual-CSS sanitizer. Never trust theme packages with
  layout-capable or exfiltration-capable CSS.

## 3. Contracts and designs worth inheriting

1. **The plugin/bridge split.** Thin in-compositor enforcement (credential
   extraction, signal forwarding, policy-cache deny) + out-of-process Go policy
   authority over a Unix socket. Validated by the Pinnacle spike
   (`research/compositor-decision.md`): the missing primitives elsewhere were
   uid attribution and synchronous input deny.
2. **The socket protocol boundary.** Everything above the plugin is
   compositor-agnostic because the bridge protocol, not the Wayfire API, is the
   interface. This is why the Wayfire exit tax is bounded at ~3k lines of C++.
   Preserve this property deliberately: the plugin is *one backend*.
3. **The whole token/identity security posture** (§2, browser gateway). Also:
   `SO_PEERCRED` stamping at the broker, append-only decision logs, root-only
   control surfaces.
4. **The evidence ladder** (see §4) and the visual-contract harness
   (`shell/desktop/visual-contracts/`, `visual-markers`); `data-surface-mode`
   markers so screenshots self-identify.
5. **Theme token validation + CSS sanitizer** as the theming trust boundary.
6. **Session-token-scoped launch lifecycle** with handle preservation and
   bounded retry discipline.
7. **The VM-first validation workflow** (`scripts/vm.sh`, phase scripts):
   privileged/compositor work happens in a disposable guest, never the host.

## 4. Hard-won platform knowledge (operational constraints)

WebKitGTK / layer-shell (the expensive lessons — most of the last month's commits):
- WebKitGTK layer-shell surfaces (`--role panel`/`overlay`) can **map while
  presenting black/no frames** on the den-k8plus Wayfire stack. `mapped` is
  never sufficient visibility evidence.
- `frame_count=0` does **not** prove a WebKitGTK surface has not rendered; the
  plugin does not always receive `frame_done` in time. Evidence ladder:
  (1) mapped/visible readback identifies the surface, insufficient alone;
  (2) `frame_count > 0` + `last_present_timestamp` is strong when available;
  (3) otherwise require a fresh capture with `visual_inspection.status ==
  "visible"`, nonzero `capture_count`/`last_capture_timestamp`, current
  `captured_at`; (4) blank/black captures are failures even when mapped.
- This forced the split-shell workarounds: toplevel-dock mode, then the native
  GTK dock (`native-shell-dock`). The empirical direction: **native chrome for
  chrome, webviews for content.**
- Surface titles used for launch matching are the *launcher* `--title` value,
  not the HTML `<title>`. Prove page identity by capture/a11y/app-command
  evidence. Prefer app-id/session/launch handles over titles entirely.
- `gtk-layer-shell` must be verified against a live Wayland session:
  `GtkLayerShell.is_supported()` can import fine and still report `False`
  without `WAYLAND_DISPLAY`. The Python GI env is pinned to `/usr/bin/python3`
  with `AGORA_WEBVIEW_PYTHON` override.

Wayfire / Wayland session:
- Wayfire refuses to run as root; sessions bootstrap via seatd + `openvt` +
  `runuser` + `dbus-run-session` with `WLR_RENDERER_ALLOW_SOFTWARE=1` in the VM
  (see README Phase 3 recipe).
- Plugin capture uses GLES readback (`wf::gles`); it fails cleanly on non-GLES
  render buffers ("snapshot buffer does not support GL readback"). Capture is
  renderer-coupled today — a reason to move to `ext-image-copy-capture-v1`.
- wlroots API churn: plugins couple to `wlr_surface` and semi-stable `wf::`
  signals; budget annual porting effort proportional to plugin size. This is the
  tax that grows with every line added to the plugin.
- Meson mis-detects clock skew when the build dir sits on the virtiofs-shared
  `/repo` mount; keep guest build dirs on guest-local FS (`/tmp/...`).

Deployment (den-k8plus):
- systemd units: `agora-wayfire`, `compositor-bridge`, `event-bus`,
  `event-bus-web`, `agora-shell-panel` (which supervises via
  `agora-shell-panel-supervisor` because `compositorctl launch` returns once the
  surface maps — systemd must supervise the supervisor, not the launch).
- Default shell mode is `split-toplevel-dock` (env `AGORA_SHELL_MODE`); the
  supervisor carries the full historical mode matrix.
- Shell config dirs: `/etc/agora-shell` (packaged defaults),
  `/var/lib/agora-shell` (runtime theme/layout).

## 5. Real consumers (what breaks if the successor gets it wrong)

- Bus topics consumed by shells and tests: `compositor.surface.*`,
  `agent.lifecycle.*`, `widget.<id>.*`, `shell.overlay.requested`,
  `webview.broadcast.*`, `webview.inbox.<uid>.*`.
- HTTP surface of `shellui` (all under `event-bus-web`): `/api/shell/state`,
  `/api/shell/session-token`, `/api/shell/apps` (+launch), `/api/shell/grants`,
  `/api/shell/escalations/decide`, `/api/shell/audit/ws`, ten
  `/api/shell/surface/*` action endpoints, `/api/shell/theme.json` (+assets/CSS),
  `/api/shell/layout.json`, `/api/shell/widget-proxy/*`, `/ws`, `/shell/` static.
- `compositorctl` command surface (sessions, launch, capture, list-surfaces,
  list-processes, terminate, input context, grants, shell install-defaults /
  install-example-widgets / list-widgets) — used by deploy scripts, the panel
  supervisor, the native dock (which shells out to compositorctl), and every
  phase test.
- `test/phase2.sh`, `test/phase3.sh` (+ `AGORA_PHASE3_HOLD=1` manual-hold mode),
  `test/phase3probe`.
- Known shell app-ids the chrome treats specially: `io.agoraos.ShellPanel`,
  `io.agoraos.ShellBackground`, `io.agoraos.ShellDock`, `io.agoraos.ShellOverlay`.

## 6. Cross-concern couplings (the seam, measured)

- Go import seam is one-directional and narrow: GUI binaries depend on
  governance only via `internal/bus` (client), `internal/schema`,
  `internal/peercred`. Zero governance→GUI imports (verified `go list -deps` +
  reverse grep at `ba4b523`).
- `internal/schema` is two concerns in one package: `compositor.go`/`layout.go`/
  `shell.go` are GUI-owned; `schema.go`/`agent_message.go`/`empirical.go` are
  governance-owned. The shell renders governance types `AgentInfo`,
  `AuditEvent`, `AdminEscalationEvent`.
- **The escalation "API" is a log file format**: `shellui` parses
  `/var/log/agent-os/admin-agent.log` (JSONL) for pending escalations and
  appends decisions to `/var/log/agent-os/admin-human-decisions.jsonl`. The
  audit tail dials the audit Unix socket directly. This works only because both
  concerns share a tree; it is the coupling the split must replace with a typed
  API.
- The wire protocol is hand-written three times: Go (`schema/compositor.go`),
  C++ (`protocol.hpp`), TS (`shell/shared/types.ts`, field-for-field mirror).
  Three-way manual drift risk; no conformance tests between them.
- `peercred` is tiny and generic: copy or publish, do not share a repo over it.

## 7. Do not inherit (the disease list)

1. **The surface-mode matrix.** `?surface=full|dock|overlay|background` URL
   multiplexing, split-shell supervisor modes, fallback modes, canary modes.
   These are fossilized workaround history around the WebKitGTK layer-shell
   defect (§4), not architecture. The successor decides the target composition
   once (see brief) and builds surfaces as first-class features.
2. **The two god files.** `internal/compositor/bridge.go` (4,460 lines) and
   `internal/shellui/server.go` (1,768 lines). Split at the move into narrow
   assignment cells.
3. **The ten copy-pasted surface-action HTTP handlers** (~55 lines each in
   `shellui`): one generic action dispatch + action table instead.
4. **The unlinted TS shell**: `strict: false`, no lint, no boundary enforcement,
   `tsc`+`cp` build, hand-rolled duck-typed bus client, hand-written protocol
   types, 1,235-line `app.ts` monolith, two divergent apps in one dir tree.
   Treat as pure predecessor evidence; rebuild on the ui-pattern v2 template
   (`den:patch/rusty-view-ui-architecture-pattern`).
5. **Chrome in the deploy directory.** `native-shell-dock` and the panel
   supervisor are products with tests owed to them, not host config.
6. **Log-file coupling** for escalations/audit (§6).
7. **Dual-source `compositor.surface.*` topics** (bridge-authoritative vs
   launcher-advisory on the same namespace). Advisory signals get their own
   namespace or die.
8. **Plugin scope creep.** Capture, input injection, stacking control, and a11y
   annotation accreted into what was budgeted as a 300–500-line enforcement
   shim. Standard protocols now cover most of these (see brief §Platform).
9. **Docs drift**: `shell/README.md` claims dist assets are "checked in" while
   git tracks nothing under `shell/dist/`; `shell/embed.go` therefore requires
   an npm build before a clean-clone `go build ./cmd/...` works. The successor
   adopts the docs-cite-real-commands check from day one.

## 8. Known defects and quirks to carry as fixtures/tests

- WebKitGTK layer-shell black-frame presentation (§4) — must exist as a live
  scenario that would have caught it.
- `frame_count=0`-while-visible — capture-readback evidence separation must be
  tested, not just documented.
- `app_not_ready` intermittent launch failures — bounded-retry behavior is
  contract, needs a test.
- Legacy layout theme overrides are ignored by default (commit `0725aa8`) —
  compatibility decision to preserve or consciously drop.
- Capture unavailable on non-GLES renderers — currently a clean error; successor
  should make renderer-independence a requirement (ext-image-copy-capture).
