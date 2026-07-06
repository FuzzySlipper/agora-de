# Webview Layer-Shell Host

This directory contains product source for the GTK4/WebKitGTK layer-shell host
used by installed shell chrome. Deployment scripts install this helper into the
operator's chosen artifact path; they do not own the helper behavior.

Supported roles:

- `background`: bottom full-work-area shell background.
- `panel`: bottom panel with an exclusive zone.
- `popup`: non-exclusive launcher/menu popup above the panel.

The agent diagnostics overlay now uses `chrome/native-overlay` so it can draw
transparent annotations without relying on WebKit background transparency.
