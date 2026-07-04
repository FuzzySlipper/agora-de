# Chrome Evidence Implications

Chrome architecture must be chosen by inspected shell evidence, not by fallback
accumulation.

## Current Preference

Use GTK4/WebKit 6 + gtk4-layer-shell for installed shell chrome on den-k8. The
GTK3/WebKit2 panel path remains useful only as a reproduction case for the old
reserve-without-paint failure.

Do not promote native chrome to product source outside `chrome/`, and do not use
`deploy/` as source-code staging.

## Option Comparison

| Option | Inspection impact | Live evidence implication |
| --- | --- | --- |
| Webview-hosted shell | DOM/model checks are useful for development, but final closure still needs compositor capture/readback. | den-k8 checks should verify shell routes, then require capture evidence for user-visible dock/panel claims. |
| GTK4/WebKitGTK layer-shell chrome | Layer-shell gives stronger panel/dock placement and input behavior than the old GTK3 panel path, but visual truth still comes from compositor/output readback. | The default installed path maps background and panel surfaces; closure uses physical output capture with expected shell-pixel classification. |
| Compositor-hosted chrome | Strongest control over geometry and input, highest compositor coupling. | Requires backend decision update plus capture/readback evidence for every promoted shell claim. |

## Scenario Mapping

The deterministic TypeScript fixtures name model scenarios that Phase 5 can map
to live checks:

- `desktop-shell-surface-controls-model-fixture`
- `operator-console-boundary-projections-model-fixture`

The chrome evidence layer should extend those with live visual scenarios:

- `den-k8-desktop-shell-dock-visible`
- `den-k8-operator-console-boundary-projections-visible`
- `den-k8-layer-shell-dock-visible` if the GTK4/WebKitGTK spike is promoted

## GTK Layer-Shell Bake-Off

`harness/live/gtk-layer-shell-bakeoff.py` runs the current den-k8 GTK3 versus
GTK4 WebKit layer-shell presentation comparison:

```bash
./harness/live/gtk-layer-shell-bakeoff.py \
  --output /tmp/agora-de-gtk-layer-shell-bakeoff.json
```

The runner launches equivalent GTK3/WebKit2/GtkLayerShell and GTK4/WebKit
6/Gtk4LayerShell background and panel cases, then records compositor readback.
On the current host GTK4 layer-shell support requires:

```text
LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so
```

The automated result can prove app id, role, layer, anchors, geometry, output,
and mapped/visible readback. It still cannot prove physical paint on this
Wayfire bridge because layer-shell capture is unavailable and `frame_count`
remains `0` for WebKit layer-shell cases.

On 2026-07-04, human observation confirmed the GTK4 panel bake-off case painted
on the physical monitor. That clears the first decision question: the black
WebKit layer-shell panel failure is not reproduced by the GTK4/WebKit 6 +
gtk4-layer-shell spike. The installed panel path now uses a repo-owned GTK4
helper with the same layer-shell configuration.

Mapped shell state alone remains insufficient. A user-visible chrome claim closes
with a physical output capture packet whose pixel classifier reports the
expected shell background, accent line, dock/panel region, and text-like pixels.

## Frame Count And Output Capture

Current layer-shell surfaces can physically paint while `frame_count` remains
`0`. The likely cause is compositor instrumentation: the Wayfire plugin observes
layer-shell `client_commit`, but the bridge only increments `frame_count` for
`frame_done`. Treat `frame_count: 0` as insufficient evidence, not as proof that
GTK4 did not paint.

The evidence ladder has grown in two directions:

- content-commit evidence for layer-shell surfaces, counted separately from true
  presented frames;
- physical output capture for the active den-k8 monitor, now the strongest
  visible-shell proof.

The intended ordering is:

1. mapped-only: insufficient for visual claims;
2. content committed: the client submitted surface content;
3. frame presented: compositor presentation metadata exists;
4. capture visible: physical output capture confirms expected shell pixels.
