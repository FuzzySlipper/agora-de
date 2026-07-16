# Agora DE Wayland Input Helper

`agora-de-wayland-input` is the owned native Wayland client that performs
widget input injection for the agent surface framework. It connects as a
Wayland client and drives `zwlr_virtual_pointer_v1` (pointer move/click) and,
later, `zwp_virtual_keyboard_v1` (type/key).

`agora-de-compositorctl input ...` verifies the target surface is tracked and
input-injectable via the compositor bridge, then execs this helper to perform
the injection. There is intentionally no shim or fallback: this helper is the
single injection engine.

## Build

Requires `wayland-scanner`, `wayland-client`, and a C compiler:

```bash
./build
```

produces `agora-de-wayland-input`.

## Scope

- Pointer: `move`, `click` (proven end-to-end on Wayfire via wlr-virtual-pointer).
- Keyboard `type`/`key`: follow-up (needs an xkb keymap; the virtual-keyboard
  protocol is vendored for that work).
