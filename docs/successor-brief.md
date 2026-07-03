# Successor Brief: agora-de

*Build prompt for the successor repo that lifts the desktop concern (compositor
mediation, shell, chrome, webview hosting, browser gateway) out of agora-os.
Written per `den:patch/successor-pattern`; the required-behavior inventory,
contracts, platform knowledge, and disease list live in the companion
`successor-lesson-packet.md` and are incorporated by reference. agora-os
returns to being the governance/isolation experiment; agora-de owns
everything a user or agent sees.*

Status: drafted 2026-07-02 against agora-os @ `ba4b523`.

---

## 1. Name and scope

**Repo name: `agora-de`.**

In scope: everything in lesson packet §1's inventory table.
Out of scope (stays in agora-os): isolation, admin-agent, audit, event-bus
broker, agent-supervisor, 3PO/R2 stack, audit-ebpf, core schema.

## 2. Boundary contract with agora-os

The runtime seam is the event bus socket plus a small typed API. The build-time
seam is a versioned protocol module. Specifically:

1. **agora-os publishes a versioned Go protocol module** containing only what
   crosses the boundary: the bus wire protocol + client, the topic namespace
   registry, and the governance types the shell renders (`AgentInfo`,
   `AuditEvent`, `AdminEscalationEvent`) plus the escalation-decision request/
   response. Everything compositor/layout/shell-shaped in `internal/schema`
   moves to agora-de.
2. **The escalation/audit log-file coupling is replaced with a typed API**
   owned by agora-os (bus request/reply or a small read socket): list pending
   escalations, submit decision, stream audit events. Interim rule if the API
   lags the split: the log parsing survives only inside one adapter package in
   agora-de, behind the same interface, with a recorded-fixture
   conformance test on the log format so drift fails CI instead of production.
3. **`peercred` is copied** (or published as a micro-module). No shared repo
   over 200 lines of helpers.
4. **Topic hygiene**: bridge-authoritative surface events keep
   `compositor.surface.*`; launcher self-reports move to an explicitly advisory
   namespace or are deleted.
5. Neither repo imports the other's internals. The protocol module is the only
   shared code, and it is versioned.

## 3. Target architecture

Two workspaces in one repo, asha-style (`/home/dev/asha` is the reference for
harness shape and assignment-cell discipline).

### Go workspace (lift-and-split — not a rewrite)

The daemons encode hard-won Wayland/WebKit behavior and carry real tests
(~3.5k lines bridge tests, 71KB shellui tests). They move, but the god files
split at the move into assignment cells with narrow APIs:

- bridge → `surfacetrack`, `launchlife`, `capture`, `input`, `policy`,
  `session`, plus a thin daemon wiring package. The compositor backend
  (Wayfire-plugin socket protocol) sits behind an interface: **the plugin is
  one backend**, so the protocol-first work in §6 and any future compositor
  have a seam to land in.
- shellui → `staticserve`, `surfaceactions` (one generic dispatch + action
  table replacing the ten copy-paste handlers), `theme`, `escalations`
  (the boundary adapter from §2), `widgets`, `catalog`.
- `harness/ci/` check scripts from day one: build, test, **depgraph check**
  (cells stay cells), docs-cite-real-commands, protocol-generation drift.

### TS workspace (full successor — the predecessor shell is evidence only)

Built on the ui-pattern v2 bootstrap template
(`den:patch/rusty-view-ui-architecture-pattern`, `den:patch/ui-pattern-bootstrap-template`),
Angular/Nx binding. Layer mapping from the predecessor:

- `protocol` — **generated from the Go schema types**; deletes
  `shell/shared/types.ts` and the three-way hand-written protocol (lesson
  packet §6).
- `transport` — WS + HTTP client with classified errors; recorded-fixture
  conformance tests against `event-bus-web`.
- `domain` — the surface/session/layout/escalation projection logic currently
  interleaved in the two `app.ts` monoliths.
- `store` — session store (token, connection) + per-feature stores;
  `AsyncState<T>` mandatory.
- `components`/`renderer` — the `desktop/widgets/*` primitives, made dumb.
- `feature-*` — taskbar, command-center, notifications, agent-health,
  escalations, audit-tail, work-surface-controls, app-launcher: one lib each.
- **Two thin shell apps composing the same feature libs**: the operator console
  and the desktop shell. This dissolves both the two-divergent-apps problem and
  the `?surface=` mode matrix — dock/overlay/background become compositions of
  feature libs, not runtime modes of a fallback monolith.
- `platform` — token storage, clock ports.
- `theme` — the `--agora-*` token system and sanitizer contract carried over.
- Live-verification harness — the phase3/capture evidence ladder formalized
  into evidence packets per the pattern's evidence classes. The WebKitGTK
  black-frame failure (lesson packet §4) is the canonical scenario the harness
  must be able to catch.

### Chrome

Native chrome is a first-class product in this repo (promoted out of
`deploy/`): the dock/panel supervisor and any native surfaces get source dirs,
tests, and CI like everything else. Whether chrome stays native or returns to
webviews is decided by Spike 1 (§6), not by default.

## 4. Explicit non-goals

- No new compositor build now. The Smithay endpoint stays a documented option
  gated on Spike 2 results (§6), not a workstream.
- No preservation of the surface-mode matrix, split-shell supervisor modes, or
  canary workarounds. Target composition is decided once, informed by Spike 1.
- No X11 support.
- No multi-machine hardware matrix; den-k8plus + VM guests remain the targets.
- No theming beyond the existing token/sanitizer contract (no theme
  marketplace, no arbitrary CSS).
- No feature growth during the lift: parity first, then new work.

## 5. Forbidden inheritance

From lesson packet §7, as hard rules for the building agents:

- No `strict: false`, no lint-free TS, no hand-written protocol types, no
  duck-typed bus clients. Protocol is generated or it doesn't exist.
- No file over ~800 lines without a planner-approved reason; the bridge and
  shellui god files do not travel intact.
- No copy-paste endpoint families; table-driven or generated.
- No product code in `deploy/`.
- No reading another concern's log files as an API (escalations/audit go
  through the §2 contract).
- No new runtime modes to route around a platform defect — a defect gets a
  spike and a decision, not a mode.
- Boundary-breaking changes stop and request planner review (the ui-pattern's
  most important sentence; unchanged here).

## 6. Platform decisions and open spikes

### Foundation: Linux + DRM/KMS + Wayland (settled, not revisited per-spike)

Wayland is the foundation by structure, not by default-ism:

- **Windows/macOS**: blocked at two independent layers — closed compositors
  (no in-compositor policy hook, which the enforcement model requires) and no
  Linux-kernel isolation primitives (uids-as-agents, cgroup slices,
  `nft meta skuid`).
- **X11**: the protocol grants every client spy/inject capability, so
  mediation can't be built on it, and Xorg is frozen upstream. Nested-X-per-
  agent (x11docker pattern) is a sandbox trick, not a DE foundation.
- **Arcan**: the only real third option — client identity and scriptable
  policy are native to it — but single-maintainer with app compat only via
  bridges. Read for prior art (SHMIF); do not build on.
- Wayland's deny-by-default is the enabling feature of the thesis: the
  compositor is the point where capability is selectively granted. This holds
  across every future in the decision tree, including a custom Smithay
  compositor, which would still speak Wayland to clients; wlroots headless
  keeps VM and display-less hosts on the same stack.

### Compositor and chrome posture

Standing decision: **keep Wayfire, dethrone WebKitGTK-as-chrome, re-thin the
plugin.** Rationale (full analysis in the review discussion, 2026-07-02):

- The Wayfire choice was evidence-based (`research/compositor-decision.md`:
  uid attribution + synchronous input deny were unavailable elsewhere) and the
  exit tax is bounded by the socket-protocol boundary at ~3.2k lines of C++.
  Not sunk cost.
- The scar tissue of the last development month (black layer-shell frames,
  `frame_count=0`, dock workarounds, native-dock fallback) is almost entirely
  **WebKitGTK-as-layer-shell-chrome** pain, not compositor pain. The repo
  already voted empirically: native chrome for chrome, webviews for content.
- The 2026 protocol landscape moved since the original spike. Of the plugin's
  six jobs, five now have standard-protocol equivalents on modern compositors:

  | Plugin job | Standard equivalent |
  |---|---|
  | uid attribution | `wp_security_context_v1` (per-agent scoped sockets) |
  | surface event stream | `ext-foreign-toplevel-list-v1` |
  | close/activate/minimize/fullscreen | `wlr-foreign-toplevel-management` |
  | capture | `ext-image-copy-capture-v1` (per-toplevel; renderer-independent, unlike the current GLES readback) |
  | input injection | virtual-keyboard/pointer, or libei |
  | **synchronous input deny** | **none — the irreducible in-compositor piece** (per-surface move/resize geometry also lacks a standard protocol) |

  Consequence: the custom enforcement core should shrink toward *deny +
  geometry*, with everything else consumed over standard protocols the bridge
  speaks directly. That cuts the annual wlroots-churn tax and cheapens any
  future compositor move, including our own.

Spikes, in value order:

1. **GTK4 + WebKitGTK 6.0 + gtk4-layer-shell presentation test** (~1 day, on
   den-k8plus). Falsifies whether the black-frame pain is GTK3-stack-era. Pass
   → split-shell webview architecture un-sticks without a native rewrite. Fail
   → the native-chrome direction is justified by evidence. Either outcome
   deletes the mode matrix from the target design.
2. **Protocol-first bridge probe** (~a few days). On a stock compositor (sway
   or niri): security-context sockets for attribution + foreign-toplevel for
   events/control + ext-image-copy-capture for capture, measured against
   `plugin.cpp`'s capability list. Deliverable: a table of portable vs
   Wayfire-only capabilities. This defines the compositor-backend interface in
   §3 regardless of outcome, and its result is the main input to the Smithay
   question (a custom compositor is a month or a year depending on how small
   the in-compositor core really is).
3. **Chrome-host bake-off — only if Spike 1 fails**: native GTK4 (extending the
   `native-shell-dock` direction) vs QtWebEngine/LayerShellQt vs WPE WebKit for
   dock/overlay surfaces. Not run preemptively.

Explicitly not spiking: other compositors in Wayfire's class (Hyprland, labwc)
— same architecture, equal-or-worse churn, no new capability. And no Smithay
build before Spike 2 reports.

## 7. Test obligations (selection environment)

Per successor doctrine, the selection environment moves with the successor or
the successor has no selection pressure:

- `test/phase2.sh`, `test/phase3.sh`, `phase3probe`, and the `scripts/vm.sh`
  GUI paths port in the first tranche and stay green throughout.
- The visual-contract harness and `data-surface-mode`/marker discipline carry
  over into the live-verification harness.
- New obligations: recorded-fixture conformance tests for the WS gateway and
  the escalation/audit adapter; protocol-generation drift checks (Go→TS, and
  at minimum conformance tests against the C++ `protocol.hpp` until it is
  generated too); depgraph check; docs-cite-real-commands.
- **Parity gate for cutover**: the predecessor shell keeps running as the
  reference implementation; the successor replaces it only when the live
  canary scenarios pass on the new stack with inspected artifacts (evidence
  classes per ui-pattern v2 — a green deterministic suite alone does not close
  the cutover claim).

## 8. Sequencing

1. Lesson packet + this brief land in Den; successor repo `agora-de`
   created from the asha harness shape + ui-pattern bootstrap template.
2. Protocol module extracted in agora-os; Go→TS protocol generation stood up.
3. Go/C++ stack moves with the split-at-move decomposition (§3); phase tests
   and VM workflow port with it; Spikes 1–2 run in parallel with the move.
4. TS shell rebuilt feature-lib by feature-lib against the generated protocol;
   predecessor shell stays live as reference.
5. Escalation/audit typed API lands in agora-os; adapter swaps under its
   conformance test.
6. Cutover on the parity gate (§7). Then **delete the predecessor GUI code
   from agora-os** — per doctrine, actually delete it; the lesson packet and
   git history are the fossil record.
