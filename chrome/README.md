# Chrome Work Area

Native chrome is product source in this repo. It does not live under `deploy/`.

- `native-dock/`: native dock/panel product work.
- `native-overlay/`: GTK4/Cairo diagnostics overlay for non-occluding
  agent-visible labels and bounds.
- `panel-supervisor/`: panel supervision source.
- `webview-layer-shell/`: GTK4/WebKitGTK layer-shell host source used by
  installed shell chrome where WebKit content is appropriate.
- `gtk4-layer-shell-spike/`: GTK4/WebKitGTK layer-shell presentation spike.
- `webview-host-spike/`: alternate webview host experiments.

The GTK4/WebKitGTK layer-shell spike is currently inspectable through:

```text
gtk4-layer-shell-spike/spike-record.json
```

It is not promoted to product source yet. The current record says native
layer-shell chrome remains a candidate until den-k8 live evidence proves it is
the more reliable dock/panel path.
