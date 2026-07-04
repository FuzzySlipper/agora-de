# Chrome Evidence Implications

Chrome architecture must be chosen by inspected shell evidence, not by fallback
accumulation.

## Current Preference

Use the current webview-hosted shell path for initial live checks on den-k8.
Keep GTK4/WebKitGTK layer-shell chrome as an inspectable candidate under
`chrome/gtk4-layer-shell-spike/` until it proves better dock/panel reliability.

Do not promote native chrome to product source outside `chrome/`, and do not use
`deploy/` as source-code staging.

## Option Comparison

| Option | Inspection impact | Live evidence implication |
| --- | --- | --- |
| Webview-hosted shell | DOM/model checks are useful for development, but final closure still needs compositor capture/readback. | den-k8 checks should verify shell routes, then require capture evidence for user-visible dock/panel claims. |
| GTK4/WebKitGTK layer-shell chrome | Layer-shell may give stronger panel/dock placement and input behavior, but visual truth still comes from compositor readback. | Promotion requires live capture packets proving dock/panel visibility and no layering/input regression. |
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

Mapped shell state alone remains insufficient. A user-visible chrome claim closes
only with a nonblank capture packet or a stronger compositor readback packet.
