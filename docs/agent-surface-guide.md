# Agent Surface Guide

> For the **human-facing** window-manager controls (keybindings, move/swap,
> layout config), see [`wm-controls.md`](./wm-controls.md). This guide covers the
> agent/compositorctl surface.

This is the agora-de framework for **agents controlling visible GUI windows**:
popping a window a human can see, reading its state, manipulating it, and
capturing evidence — with the security ceremony intentionally **off** for now
so real-app GUI testing is unblocked. The ceremony has a documented seam to
switch back on later (`governed` launch policy).

This guide is the authority for the agent path. `docs/deployed-wm-model.md`
covers the broader deployed WM model and the human shell controls.

## Design intent (read this first)

Testing whether agents can actually drive real GUIs is the current priority.
Front-loading session tokens, audit ids, and launch allowlists has historically
cost more time (agents juggling tokens) than it saved, while the underlying
mechanism was still being proved out. So:

- **Native launch is open by default.** An agent can pop a native app window
  with no session token, no audit id, and no allowlist.
- The token/audit fields still exist on the launch contract, but they are
  **recorded, not enforced**. The compositor bridge does not authorize launches.
- The `governed` launch policy (below) is the single seam to restore ceremony
  later, intended as the hook for an agora-os governance peer. It is opt-in, not
  the default.
- If you are an agent working in this repo and you find launch requiring tokens,
  treat that as drift against this intent, not the intended shape.

## The agent API: `agora-de-compositorctl`

`agora-de-compositorctl` is the agent surface-control tool. It speaks to the compositor
bridge over `/run/agent-os/compositor-control.sock` (group `agents`, mode
`0660`). Any process in the `agents` group can use it.

> **Binary name — important.** The tool is installed as `agora-de-compositorctl`,
> not bare `compositorctl`. The predecessor agora-os `compositorctl` (legacy
> `--cmd` / `launch_app`, incompatible with this bridge) is **superseded and
> removed** by `install-compositor-bridge-service.sh`, so bare `compositorctl`
> does not exist — always use the explicit `agora-de-compositorctl` name. There
> is intentionally no shim or fallback.

```
agora-de-compositorctl [--pretty] <command> [flags]
  launch         Launch a native process or webview window
  list-surfaces  List tracked compositor surfaces
  layout         Read or change structured layout state
  output         List outputs or capture a physical output
  surface        Focus, close, or request layout actions for a surface
  workspace      Request workspace actions
```

**Model rule:** agents treat structured state as authority and screenshots as
evidence. Target surfaces by **stable surface id** and **zone label** returned
by `list-surfaces` / `layout get` — never by guessing from pixels or browser
rectangles.

## The loop: pop → see → manipulate → capture

### 1. Pop a visible window (human sees it tile into a zone)

Native app — **no ceremony** (open policy default):

```bash
agora-de-compositorctl launch --arg alacritty --wait-surface
# → { "launch_id": "...", "pid": ..., "surface_id": "view-42", "status": "launched" }
```

- `--uid`/`--gid` default to the calling process (you), so the app runs as the
  agent. Override only when rehearsing a different user.
- `--arg` repeats to build the argv: `--arg alacritty --arg -e --arg top`.
- `--env KEY=VALUE` repeats for environment; `--cwd` sets the working dir.
- `--wait-surface` blocks until the surface maps and returns its `surface_id`.
  `--wait-timeout-ms` (default 5000) bounds the wait.

Webview window (URL or local HTML) — also no ceremony:

```bash
agora-de-compositorctl launch \
  --url https://example.com \
  --app-id io.agent.report \
  --webview-title "Agent Report" \
  --width 900 --height 700 \
  --wait-surface
```

### 2. See it (structured state first, pixels second)

```bash
agora-de-compositorctl list-surfaces                       # surface_id, app_id, title, mapped, focused
agora-de-compositorctl layout get                          # mode, zones, surfaces, geometry, focus/order, labels
curl -s http://127.0.0.1:17780/api/surfaces | jq  # lifecycle + owner uid over HTTP
agora-de-compositorctl output capture --name HDMI-A-1      # physical screenshot for evidence
```

The `?surface=overlay` shell layer renders stable numbered labels, app/title
badges, focus state, zone hints, and geometry bounds over native app surfaces,
so a human (or capture classifier) can see exactly which surface an agent is
acting on.

### 3. Manipulate it (target by surface id / zone)

```bash
agora-de-compositorctl surface focus   --surface view-42
agora-de-compositorctl surface close   --surface view-42
agora-de-compositorctl surface tile    --surface view-42 --zone primary
agora-de-compositorctl surface assign-zone --surface view-42 --zone secondary
agora-de-compositorctl surface set-floating --surface view-42
agora-de-compositorctl surface promote --surface view-42     # promote into master/primary
agora-de-compositorctl surface maximize --surface view-42
agora-de-compositorctl surface minimize --surface view-42
agora-de-compositorctl surface fullscreen --surface view-42
agora-de-compositorctl surface move-resize --surface view-42 # backend applies negotiated geometry
agora-de-compositorctl layout set-mode --mode zones           # freeform | zones | columns
agora-de-compositorctl workspace activate --workspace workspace-1
```

### 3b. Drive widget input (pointer)

Inject pointer events into a tracked, input-injectable surface (non-shell
surfaces are rejected as a guardrail). Coordinates are output-relative — derive
them from the surface geometry in `layout get`. Output extents are auto-resolved
from the bridge.

```bash
# click (left button) at the surface's center
agora-de-compositorctl input pointer click --surface view-42 --x 640 --y 360
# move/warp the pointer without clicking
agora-de-compositorctl input pointer move   --surface view-42 --x 640 --y 360
# other buttons: --button 0x111 (BTN_RIGHT), 0x112 (BTN_MIDDLE)
```

The `input` subcommand verifies the surface is tracked and `input_injectable`
(via the bridge), then drives the owned `agora-de-wayland-input` helper, which
injects through `wlr-virtual-pointer`. It returns a structured result embedding
the helper output.

### 3c. Drive widget input (keyboard)

Type text or send key events into a tracked, input-injectable surface. The
`type` action auto-selects the text-entry protocol: it commits text via the
owned **input-method** path (`zwp_input_method_v1` / `text-input-v3`) for
clients like **Chromium**, and falls back to the virtual-keyboard engine
(`wtype`) for native wl_keyboard clients (Alacritty, foot, GTK). The `key`
action (Return, Escape, ...) uses virtual-keyboard. The same tracked-surface
guardrail applies (non-shell surfaces only).

```bash
# type text (auto: input-method for Chromium, virtual-keyboard for native)
agora-de-compositorctl input keyboard type --surface view-42 --text "echo hi"
# force a method: input-method | virtual-keyboard
agora-de-compositorctl input keyboard type --surface view-42 --text "hi" --method input-method
# send a named key (Return, Escape, Tab, space, BackSpace, ...)
agora-de-compositorctl input keyboard key  --surface view-42 --key Return
```

The input-method path needs the text-input field focused (enabled), so
compositorctl binds the input method first, then clicks `--click-x/--click-y`
(default: output center) to focus the field, then commits. Provide `--click-x`
`--click-y` when the field is not at the output center. **Requires Wayfire's
`input-method-v1` plugin enabled** (`enable_text_input_v3=true` in
`wayfire.ini`) — it advertises `zwp_input_method_v1` / `zwp_text_input_v3`.

The owned `agora-de-wayland-input` C helper owns the input-method path; the
virtual-keyboard path drives `wtype` (install it for native-app typing). An
owned virtual-keyboard engine is retained as source but hits an unresolved
Wayfire quirk (see Known gaps / task #5665).

Equivalent HTTP routes exist for shell/agent use:

- `POST /api/surfaces/action` — `focus`, `close`, `maximize`, `minimize`, `fullscreen`.
- `POST /api/layout/action` — `setMode`, `assignZone`, `tile`, `moveResize`,
  `setFloating`, `activateWorkspace`.

## Launch policy: the open/governed seam

`agora-de-compositorctl launch` reads `AGORA_DE_AGENT_LAUNCH_POLICY`:

| value       | native launch ceremony                                                                   |
| ----------- | ---------------------------------------------------------------------------------------- |
| `open`      | **default.** No `--session-token` or `--audit-correlation-id` required. Apps run as the calling agent. |
| `governed`  | `--session-token` and `--audit-correlation-id` are required for native launches. The future hook for per-agent identity, audit, and agora-os governance handoff. |

Webview launches (`--url`/`--path`) never require tokens under either policy.

To rehearse the governed path without changing code:

```bash
AGORA_DE_AGENT_LAUNCH_POLICY=governed agora-de-compositorctl launch --arg alacritty \
  --session-token session-1 --audit-correlation-id audit-1 --wait-surface
```

The bridge still does not authorize in `governed` today — that enforcement is
the future agora-de work that pairs with an agora-os peer. The flag exists so
agents and tests can be written against the eventual contract now.

## What lives where (the split)

- **agora-de** owns the DE surface: compositor bridge, layout state, shellui,
  and this launch/surface-control framework. It does not own agent identity,
  per-uid isolation, or audit authority.
- **agora-os** keeps isolation, admin-agent, audit, event-bus, and
  agent-supervisor. The `governed` policy is where an agora-os peer will later
  hand validated tokens/identity into an agora-de launch.
- Access control today is filesystem only: the `agents` group can reach the
  compositor control socket. There is no per-agent identity enforcement inside
  agora-de — by design, until the governed path is wired.

## HTTP path (human launcher, not the agent framework)

`shellui` `POST /api/catalog/launch` is the **human** launcher backing. Native
entries there are gated by `AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST` (off by
default; `*` or `agora-de-native-launch-config enable-all` opens it). Agents
should prefer `agora-de-compositorctl launch` (this guide), which is ceremony-free under
the open policy and does not depend on shellui's allowlist.

## Known gaps

This framework covers window **lifecycle, layout, capture, pointer input
(move/click), and keyboard input (type/key)** — including **Chromium text
entry** via input-method/text-input-v3. Outstanding gaps:

- **Owned virtual-keyboard engine:** the native-app `type`/`key` path currently
drives `wtype`. An owned `agora-de-wayland-input` virtual-keyboard path is
retained as source but hits an unresolved Wayfire rejection of byte-identical
requests (task #5665). The input-method path (Chromium) is owned.
- **Accessibility/semantic** queries are not wired.

This is called out so agents don't attempt the legacy `--cmd`/`launch_app`/`click`
path against this bridge.

## Proving it works

End-to-end native launch + readback + capture evidence:

```bash
./harness/live/check-native-launch.py --output-name HDMI-A-1 --require-capture
./harness/live/check-shell-loop.py   --base-url http://127.0.0.1:17780 --output-name HDMI-A-1 --require-capture
./harness/live/check-overlay-labels.py --output-name HDMI-A-1 --require-capture
./harness/live/check-auto-tiling-wm.py --base-url http://127.0.0.1:17780 \
  --app-id Alacritty.desktop --expected-app-id Alacritty --output-name HDMI-A-1 --require-capture
```

Failure classes the harnesses distinguish: `launch`, `visibility`,
`planner-mismatch`, `backend-placement`, `occlusion`, `focus-order`,
`shell-action`, `agent-action`, `overlay`, `capture`, `restart`, `cleanup`.
