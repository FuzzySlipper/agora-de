# Native Diagnostics Overlay

GTK4/Cairo layer-shell overlay used for agent-visible diagnostics. It draws
window bounds, stable labels, focus state, and zone hints directly from
`/api/layout` and `/api/surfaces`.

This helper intentionally does not host WebKit content. Its window clears to a
transparent Cairo surface and only paints annotation pixels, so native XDG app
content remains visible in physical output captures.
